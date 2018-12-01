package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
)

var (
	wssURL      = flag.String("url", "", "Web socket address example: wss://doppler.system.domain:443/apps/41abc841-cbc8-4cab-854d-640a7c8b6a5f/stream")
	apiTarget   = flag.String("api", "", "CF API endpoint https://api.system.domain.com")
	accessToken = flag.String("token", "", "Provide an access token used to authenticate with doppler endpoint. Defaults to ~/.cf/config.json")
	logger      *log.Logger
	cfconf      CFConfig
	mc          Metrics
)

const (
	TrafficControllerJob = "loggregator_trafficcontroller"
	DopplerJob           = "doppler"
	SyslogAdapterJob     = "syslog_adapter"
	SyslogSchedulerJob   = "syslog_scheduler"
	MetronOrigin         = "loggregator.metron"
)

// CFConfig struct used to parse ~/.cf/config.json
type CFConfig struct {
	AccessToken string `json:"AccessToken"`
	Target      string `json:"Target"`
	WSSURL      string
}

// Get access token from ~/.cf/config.json
func getCFConfig() (CFConfig, error) {
	if *accessToken != "" && *apiTarget != "" {
		return CFConfig{AccessToken: *accessToken, Target: *apiTarget}, nil
	}
	conf := CFConfig{}
	usr, err := user.Current()
	if err != nil {
		return conf, fmt.Errorf("Could not get users home directory: %s", err)
	}
	config := path.Join(usr.HomeDir, ".cf/config.json")
	_, err = os.Stat(config)
	if err != nil {
		return conf, fmt.Errorf("Unalbe to stat %s: %s", config, err)
	}

	b, err := ioutil.ReadFile(config)
	if err != nil {
		return conf, fmt.Errorf("Reading Config %s failed: %s", config, err)
	}
	err = json.Unmarshal(b, &conf)
	if err != nil {
		return conf, fmt.Errorf("Could not parse config %s: %s", config, err)
	}
	if conf.AccessToken == "" {
		return conf, fmt.Errorf("Invalid access token found in %s", config)
	}
	return conf, nil
}

func (c *CFConfig) setDopplerEndpoint() error {
	type apiInfoResp struct {
		DopplerEndpoint string `json:"doppler_logging_endpoint"`
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(fmt.Sprintf("%s/v2/info", c.Target))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Invalid API Response Code: %d\n%s", resp.StatusCode, string(bodyBytes))
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	info := apiInfoResp{}
	err = json.Unmarshal(bodyBytes, &info)
	if err != nil {
		return err
	}
	c.WSSURL = info.DopplerEndpoint + "/firehose/firehose-analyzer"
	return nil
}

func createSocket() (*websocket.Conn, error) {
	badConn := new(websocket.Conn)
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	h := http.Header{}
	h.Add("Authorization", cfconf.AccessToken)
	conn, resp, err := dialer.Dial(cfconf.WSSURL, h)
	if err != nil {
		var errString error
		if resp != nil {
			errString = fmt.Errorf("HTTP Response: %s\nWEBSOCKET Error: %s", resp.Status, err)
		} else {
			errString = fmt.Errorf("WEBSOCKET Error:%s", err)
		}
		return badConn, errString
	}
	return conn, nil
}

func main() {
	//var buf bytes.Buffer
	logger = log.New(os.Stdout, "logger: ", log.Ldate|log.Ltime|log.Lshortfile)

	flag.Parse()
	var err error
	cfconf, err = getCFConfig() // TODO need to handle refresh token given access token can expire pretty fast
	if err != nil {
		logger.Fatalln(err)
	}

	// get doppler endpoint
	err = cfconf.setDopplerEndpoint()
	if err != nil {
		logger.Fatalln(err)
	}

	conn, err := createSocket()
	if err != nil {
		logger.Fatalf("failed to connect to %s: %s", cfconf.WSSURL, err)
	}
	defer conn.Close()

	mc = Metrics{}
	input := make(chan []byte, 5000)
	output := make(chan *events.Envelope, 10000)
	dn := dropsonde_unmarshaller.NewDropsondeUnmarshaller()

	fmt.Println("starting dropsnode unmarshaller...")
	go dn.Run(input, output)

	fmt.Println("starting output collector...")
	go func(output chan *events.Envelope) {
		for {
			select {
			case e := <-output:
				mc.parseEnvelope(e)
			}
		}
	}(output)

	fmt.Println("starting read loop...")
	go loopTerm()
	for {
		_, p, err := conn.ReadMessage()
		if err != nil {
			logger.Println(err)
			time.Sleep(1 * time.Second)
		}
		input <- p
	}

}
