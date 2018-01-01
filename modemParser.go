package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/grahamgreen/goutils"
	"github.com/matryer/try"
	"gopkg.in/urfave/cli.v1"

	log "github.com/sirupsen/logrus"
)

type Downstream struct {
	Name           string
	DCID           string
	Freq           string
	Power          float64
	SNR            float64
	Modulation     string
	Octets         int64
	Correcteds     int64
	Uncorrectables int64
}

type Upstream struct {
	Name        string
	UCID        int64
	Freq        float64
	Power       float64
	ChannelType string
	SymbolRate  int64
	Modulation  string
}

type Status struct {
	Uptime   time.Duration
	DateTime time.Time
}

type Record struct {
	DS   []Downstream
	US   []Upstream
	Stat Status
}

func ParseDS(ds []string) Downstream {
	var theDS Downstream
	var err error
	theDS.Name = ds[0]
	theDS.DCID = strings.TrimSpace(ds[1])
	freqHolder := strings.Split(ds[2], " ")
	theDS.Freq = freqHolder[1]
	powerHolder := strings.Split(ds[3], " ")
	theDS.Power, err = strconv.ParseFloat(powerHolder[1], 64)
	goutils.Check(err)
	snrHolder := strings.Split(ds[4], " ")
	theDS.SNR, err = strconv.ParseFloat(snrHolder[1], 64)
	goutils.Check(err)
	theDS.Modulation = ds[5]
	theDS.Octets, err = strconv.ParseInt(strings.TrimSpace(ds[6]), 10, 64)
	goutils.Check(err)
	theDS.Correcteds, err = strconv.ParseInt(strings.TrimSpace(ds[7]), 10, 64)
	goutils.Check(err)
	theDS.Uncorrectables, err = strconv.ParseInt(strings.TrimSpace(ds[8]), 10, 64)
	goutils.Check(err)

	return theDS
}

func ParseUS(us []string) Upstream {
	var theUS Upstream
	var err error
	theUS.Name = us[0]
	theUS.UCID, err = strconv.ParseInt(strings.TrimSpace(us[1]), 10, 64)
	goutils.Check(err)
	freqHolder := strings.Split(us[2], " ")
	theUS.Freq, err = strconv.ParseFloat(freqHolder[1], 64)
	goutils.Check(err)
	powerHolder := strings.Split(us[3], " ")
	theUS.Power, err = strconv.ParseFloat(powerHolder[1], 64)
	goutils.Check(err)
	theUS.ChannelType = us[4]
	srHolder := strings.Split(us[5], " ")
	theUS.SymbolRate, err = strconv.ParseInt(strings.TrimSpace(srHolder[1]), 10, 64)
	goutils.Check(err)
	theUS.Modulation = us[6]

	return theUS

}

func CleanString(s string) string {
	//remove spaces
	s = strings.TrimSpace(s)
	//remove last char
	s = s[:len(s)-1]
	s = strings.TrimSpace(s)

	return s
}

func ArrisScrape() (int, error) {
	loc, _ := time.LoadLocation("Local")
	var lineHolder bytes.Buffer
	var theDS Downstream
	var theUS Upstream
	var status Status
	var doc *goquery.Document
	err := try.Do(func(attempt int) (bool, error) {
		var err error
		doc, err = goquery.NewDocument("http://192.168.100.1/cgi-bin/status_cgi")
		if err != nil {
			log.Error(err)
			time.Sleep(2 * time.Second)
		}
		return attempt < 5, err
	})
	if err != nil {
		return 1, fmt.Errorf("could not reach modem")

	}

	doc.Find("h4").Each(func(i int, s *goquery.Selection) {
		if s.Text() == " Downstream " {
			s.Next().Find("tr").Each(func(j int, s2 *goquery.Selection) {
				if len(s2.Find("td").First().Text()) > 0 {
					//fmt.Printf("%s\n", s2.Text())
					s2.Find("td").Each(func(k int, s3 *goquery.Selection) {
						lineHolder.WriteString(s3.Text() + ", ")
						//fmt.Printf("%s, ", s3.Text())
					})
					theSplitLine := strings.Split(lineHolder.String(), ",")
					theDS = ParseDS(theSplitLine)
					log.WithFields(log.Fields{
						"name":           theDS.Name,
						"dcid":           theDS.DCID,
						"freq":           theDS.Freq,
						"power":          theDS.Power,
						"snr":            theDS.SNR,
						"modulation":     theDS.Modulation,
						"octets":         theDS.Octets,
						"correcteds":     theDS.Correcteds,
						"uncorrectables": theDS.Uncorrectables,
					}).Info("downstream")
					lineHolder.Reset()
				}
			})
		}
		if s.Text() == " Upstream " {
			s.Next().Find("tr").Each(func(j int, s2 *goquery.Selection) {
				if len(s2.Find("td").First().Text()) > 0 {
					//fmt.Printf("%s\n", s2.Text())
					s2.Find("td").Each(func(k int, s3 *goquery.Selection) {
						lineHolder.WriteString(s3.Text() + ", ")
						//fmt.Printf("%s, ", s3.Text())
					})
					theSplitLine := strings.Split(lineHolder.String(), ",")
					theUS = ParseUS(theSplitLine)
					log.WithFields(log.Fields{
						"name":        theUS.Name,
						"ucid":        theUS.UCID,
						"freq":        theUS.Freq,
						"power":       theUS.Power,
						"channelType": theUS.ChannelType,
						"modulation":  theUS.Modulation,
					}).Info("upstream")
					lineHolder.Reset()
				}
			})

		}
	})
	//this sucks and searches the td's twice
	doc.Find("td").Each(func(i int, s *goquery.Selection) {
		if s.Text() == "System Uptime: " {
			//fmt.Println(s.Next().Text())
			poo := strings.Split(s.Next().Text(), ":")
			d, h, m := poo[0], poo[1], poo[2]
			dInt, err := strconv.ParseInt(CleanString(d), 10, 64)
			goutils.Check(err)
			hInt, err := strconv.ParseInt(CleanString(h), 10, 64)
			goutils.Check(err)
			mInt, err := strconv.ParseInt(CleanString(m), 10, 64)
			goutils.Check(err)
			hInt += dInt * 24
			dString := fmt.Sprintf("%dh%dm", hInt, mInt)
			dur, err := time.ParseDuration(dString)
			goutils.Check(err)
			status.Uptime = dur
		}
		if s.Text() == "Time and Date:" {
			tString := s.Next().Text()
			t, err := time.ParseInLocation("Mon 2006-01-02 15:04:05", tString, loc)
			goutils.Check(err)
			status.DateTime = t
		}
	})
	log.WithFields(log.Fields{
		"uptime":   status.Uptime,
		"dateTime": status.DateTime,
	}).Info("status")
	return 0, nil
}

func main() {
	var logfile string
	app := cli.NewApp()
	app.Name = "Modem Parse"
	app.Usage = "Parse modem stats page and graph results"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "logfile, l",
			Value:       "/var/log/modemparse.log",
			Usage:       "Log output to `FILE`",
			Destination: &logfile,
		},
	}
	app.Action = func(context *cli.Context) {
		f, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Errorf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(f)
		log.SetLevel(log.InfoLevel)

		log.Info("Starting ModemParse")

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		signal.Notify(c, syscall.SIGTERM)
	loop:
		for {
			select {
			case <-c:
				fmt.Println("got interrupted")
				log.Println("Stopping Scrape")
				break loop
			default:
				ret, err := ArrisScrape()
				if err != nil {
					log.WithFields(log.Fields{
						"return": ret,
						"error":  err,
					}).Error("modem down")
					fmt.Printf("%v %v", ret, err)
					time.Sleep(1 * time.Second)
				} else {
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
		signal.Stop(c)
	}
	app.Run(os.Args)
}
