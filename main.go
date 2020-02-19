package main

import (
	"encoding/json"
	"github.com/jessevdk/go-flags"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"time"
)

type Options struct {
	Verbose bool `long:"verbose" short:"v"`
	Watch   bool `long:"watch" description:"Watch for interface changes"`

	// Netlink Interface
	Interface       string `long:"interface" default:"tun0" short:"i" value-name:"IFACE" description:"Use address from interface"`
	InterfaceFamily Family `long:"interface-family" default:"ipv6" value-name:"ipv4|ipv6|all" description:"Limit to interface addreses of given family"`

	// DNS Update
	Server        string        `long:"server" value-name:"HOST[:PORT]" description:"Server for UPDATE query, default is discovered from zone SOA"`
	Timeout       time.Duration `long:"timeout" value-name:"DURATION" default:"10s" description:"Timeout for sever queries"`
	Retry         time.Duration `long:"retry" value-name:"DURATION" default:"30s" description:"Retry interval, increased for each retry attempt"`
	TSIGName      string        `long:"tsig-name" default:"nodes.bonuscloud.work" value-name:"FQDN"`
	TSIGSecret    string        `long:"tsig-secret" value-name:"BASE-64" env:"TSIG_SECRET" description:"base64-encoded shared TSIG secret key"`
	TSIGAlgorithm TSIGAlgorithm `long:"tsig-algorithm" default:"hmac-md5" value-name:"hmac-{md5,sha1,sha256,sha512}" default:"hmac-sha1."`
	Zone          string        `long:"zone" default:"nodes.bonuscloud.work." value-name:"FQDN" description:"Zone to update, default is derived from name"`
	TTL           time.Duration `long:"ttl" value-name:"DURATION" default:"60s" description:"TTL for updated records"`
	Hostname      string        `long:"hostname" value-name:"FQDN" description:"HostName to bind"`

	Args struct {
		Name string `value-name:"FQDN" description:"DNS Name to update"`
	} `positional-args:"yes"`
}

const (
	SerializeFile = "/opt/bcloud/node.db"
	TSIG          = "/mpNYBgjQUD1ZY9lFRGDabdZu0jxypHIJCI4HquSeEL1IVeuqB6rsc/wBLATpG8XngZHJBCSgkUWfRbjPL/MIA=="
)

type Node struct {
	Bcode string `json:"bcode"`
	Email string `json:"email"`
}

func main() {
	var options Options

	if _, err := flags.Parse(&options); err != nil {
		log.Fatalf("flags.Parse: %v", err)
		os.Exit(1)
	}

	if options.Args.Name == "" {
		j, err := ioutil.ReadFile(SerializeFile)
		if err != nil {
			log.Fatalf("fail to get name")
			os.Exit(1)
		}

		if len(j) == 0 {
			log.Fatalf("fail to get name")
			os.Exit(1)
		}
		var node Node
		err = json.Unmarshal(j, &node)
		if err != nil {
			log.Fatalf("fail to get name")
			os.Exit(1)
		}
		options.Args.Name = node.Bcode + "." + options.Zone
	}
	log.Printf("get name %s", options.Args.Name)

	var update = Update{
		ttl:     int(options.TTL.Seconds()),
		timeout: options.Timeout,
		retry:   options.Retry,
		verbose: options.Verbose,
	}

	if options.Hostname == "" {
		Hostname, _ := os.Hostname()
		match, _ := regexp.MatchString("^(dc|ms|edge)-", Hostname)
		if match == true {
			options.Hostname = Hostname + "." + options.Zone
			log.Printf("get hostname %s", options.Hostname)
		}
	}

	if err := update.Init(options.Args.Name, options.Zone, options.Server, options.Hostname); err != nil {
		log.Fatalf("init: %v", err)
	}

	if options.TSIGSecret == "" {
		options.TSIGSecret = TSIG
	}
	if options.TSIGSecret != "" {
		var name = options.TSIGName

		if name == "" {
			name = options.Args.Name
		}

		log.Printf("using TSIG: %v (algo=%v)", name, options.TSIGAlgorithm)

		update.InitTSIG(name, options.TSIGSecret, options.TSIGAlgorithm)
	}

	// addrs
	addrs, err := InterfaceAddrs(options.Interface, options.InterfaceFamily)
	if err != nil {
		log.Fatalf("addrs scan: %v", err)
	}

	// update
	update.Start()

	for {
		log.Printf("update...")

		if err := update.Update(addrs); err != nil {
			log.Fatalf("update: %v", err)
		}

		log.Printf("wait...")

		if !options.Watch {
			break
		}

		if err := addrs.Read(); err != nil {
			log.Fatalf("addrs read: %v", err)
		} else {
			log.Printf("addrs update...")
		}
	}
	if err := update.Done(); err != nil {
		log.Printf("update done: %v", err)
	} else {
		log.Printf("update done")
	}

}
