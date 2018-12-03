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
	"os/exec"
	"os/user"
	"path"
	"time"

	"github.com/cloudfoundry/dropsonde/dropsonde_unmarshaller"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gorilla/websocket"
)

var (
	wssURL         = flag.String("url", "", "Web socket address example: wss://doppler.system.domain:443")
	apiTarget      = flag.String("api", "", "CF API endpoint https://api.system.domain.com")
	accessToken    = flag.String("token", "", "Provide an access token used to authenticate with doppler endpoint. Defaults to ~/.cf/config.json")
	outFile        = flag.String("o", "", "Specifiy an output file that records data in csv format")
	logger         *log.Logger
	cfconf         CFConfig
	mc             Metrics
	arvhiveEnabled = false
	ofh            *os.File
)

const (
	// TrafficControllerJob name of traffic controller job
	TrafficControllerJob = "loggregator_trafficcontroller"
	// DopplerJob name of doppler job
	DopplerJob = "doppler"
	// SyslogAdapterJob name of syslog adapter job
	SyslogAdapterJob = "syslog_adapter"
	// SyslogSchedulerJob name of syslog scheduler job
	SyslogSchedulerJob = "syslog_scheduler"
	// MetronOrigin is the origin label of the metron agent
	MetronOrigin = "loggregator.metron"
)

// CFConfig struct used to parse ~/.cf/config.json
type CFConfig struct {
	AccessToken string `json:"AccessToken"`
	Target      string `json:"Target"`
	WSSURL      string
}

// Get access token from ~/.cf/config.json
func (c *CFConfig) getCFConfig() error {
	if *accessToken != "" && *apiTarget != "" {
		c.AccessToken = *accessToken
		c.Target = *apiTarget
		return nil
	}
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("Could not get users home directory: %s", err)
	}
	config := path.Join(usr.HomeDir, ".cf/config.json")
	_, err = os.Stat(config)
	if err != nil {
		return fmt.Errorf("Unalbe to stat %s: %s", config, err)
	}

	b, err := ioutil.ReadFile(config)
	if err != nil {
		return fmt.Errorf("Reading Config %s failed: %s", config, err)
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return fmt.Errorf("Could not parse config %s: %s", config, err)
	}
	if c.AccessToken == "" {
		return fmt.Errorf("Invalid access token found in %s", config)
	}
	return nil
}

// setDopplerEndpoint use cf cli to get the api info.  This will force a refresh of the access token and prevent 401 errors.
func (c *CFConfig) setDopplerEndpoint() error {
	type apiInfoResp struct {
		DopplerEndpoint string `json:"doppler_logging_endpoint"`
	}

	cfcli, err := exec.LookPath("cf")
	if err != nil {
		return fmt.Errorf("cf cli lookup failed: %s", err)
	}
	bodyBytes, err := exec.Command(cfcli, "curl", "/v2/info").Output()
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
	// get doppler endpoint
	cfconf = CFConfig{}
	err := cfconf.setDopplerEndpoint()
	if err != nil {
		logger.Fatalln(err)
	}

	err = cfconf.getCFConfig() // TODO need to handle refresh token given access token can expire pretty fast
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

	if *outFile != "" {
		logger.Printf("starting to archive output to %s", *outFile)
		var err error
		ofh, err = os.Create(*outFile)
		if err != nil {
			logger.Fatalln(err)
		}
		ofh.Write([]byte(fmt.Sprintf("time,job/index,metric,value,type,unit\n")))
		arvhiveEnabled = true
		defer ofh.Close()
	}

	logger.Println("starting dropsnode unmarshaller...")
	go dn.Run(input, output)

	logger.Println("starting output collector...")
	go func(output chan *events.Envelope) {
		for {
			select {
			case e := <-output:
				mc.parseEnvelope(e)
			}
		}
	}(output)

	logger.Println("starting read loop...")
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
