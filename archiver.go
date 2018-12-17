package main

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
)

//time,job/index,metric,value,type,unit
func archiveMetric(e *events.Envelope, m string, v interface{}, u interface{}) {
	_, err := ofh.Write([]byte(fmt.Sprintf("%s,%s,%s/%s,%s,%v,%s,%v\n",
		time.Unix(e.GetTimestamp()/1000000000, 0).Format(time.RFC3339),
		e.GetOrigin(),
		e.GetJob(),
		e.GetIndex(),
		m,
		v,
		e.GetEventType(),
		u,
	)))

	if err != nil {
		logger.Fatalln(err)
	}
}
