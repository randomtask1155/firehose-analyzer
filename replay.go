package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/sonde-go/events"
)

func runReplay() {
	fh, err := os.Open(*replay)
	if err != nil {
		logger.Fatalln(err)
	}
	defer fh.Close()

	timestamp := 0
	origin := 1
	jobIndex := 2
	metric := 3
	value := 4
	eventType := 5
	unit := 6

	totalLines := 0.0
	lineScanner := bufio.NewScanner(fh)
	for lineScanner.Scan() {
		totalLines++
	}
	_, err = fh.Seek(0, 0)
	if err != nil {
		logger.Printf("could not rewinde file: %s", err)
	}
	currentLine := 0.0
	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		currentLine++
		d := strings.Split(scanner.Text(), ",")
		if len(d) != 7 {
			logger.Printf("Bad Data in line : %v", d)
			continue
		}

		if d[timestamp] == "time" {
			continue // skip header
		}

		ts, err := time.Parse("2006-01-02T15:04:05-07:00", d[timestamp])
		if err != nil {
			logger.Printf("Can not parse timestamp: %s", err)
			continue
		}
		tu := ts.Unix()
		var et events.Envelope_EventType
		var e *events.Envelope

		ji := strings.Split(d[jobIndex], "/")
		if len(ji) != 2 {
			logger.Println("Bad job index data")
			continue
		}
		if d[eventType] == "ValueMetric" {
			val, err := strconv.ParseFloat(d[value], 64)
			if err != nil {
				logger.Printf("parsing value failed: %v: %s", d, err)
				continue
			}
			et = events.Envelope_ValueMetric
			e = &events.Envelope{
				Timestamp: &tu,
				Origin:    &d[origin],
				Job:       &ji[0],
				Index:     &ji[1],
				EventType: &et,
				ValueMetric: &events.ValueMetric{
					Name:  &d[metric],
					Value: &val,
					Unit:  &d[unit],
				},
			}
		} else {
			var v, u int
			var err error
			var vv, uu uint64
			v, err = strconv.Atoi(d[value])
			if err != nil {
				logger.Println(err)
				continue
			}
			u, err = strconv.Atoi(d[unit])
			if err != nil {
				logger.Println(err)
				continue
			}
			vv = uint64(v)
			uu = uint64(u)
			et = events.Envelope_CounterEvent
			e = &events.Envelope{
				Timestamp: &tu,
				Origin:    &d[origin],
				Job:       &ji[0],
				Index:     &ji[1],
				EventType: &et,
				CounterEvent: &events.CounterEvent{
					Name:  &d[metric],
					Delta: &vv,
					Total: &uu,
				},
			}
		}
		updateProgressBar(currentLine / totalLines)
		mc.parseEnvelope(e)
		if *speed > 0 {
			time.Sleep(time.Duration(100 / *speed) * time.Millisecond)
		}

		//TODO CONFIGURE SPEED
	}
	updateProgressBar(1)
}
