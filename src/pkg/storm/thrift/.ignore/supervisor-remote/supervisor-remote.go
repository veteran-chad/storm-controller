// Code generated by Thrift Compiler (0.22.0). DO NOT EDIT.

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	thrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/veteran-chad/storm-controller/pkg/storm/thrift"
)

var _ = github.com/veteran-chad/storm-controller/pkg/storm/thrift.GoUnusedProtection__

func Usage() {
	fmt.Fprintln(os.Stderr, "Usage of ", os.Args[0], " [-h host:port] [-u url] [-f[ramed]] function [arg1 [arg2...]]:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nFunctions:")
	fmt.Fprintln(os.Stderr, "  void sendSupervisorAssignments(SupervisorAssignments assignments)")
	fmt.Fprintln(os.Stderr, "  Assignment getLocalAssignmentForStorm(string id)")
	fmt.Fprintln(os.Stderr, "  void sendSupervisorWorkerHeartbeat(SupervisorWorkerHeartbeat heartbeat)")
	fmt.Fprintln(os.Stderr)
	os.Exit(0)
}

type httpHeaders map[string]string

func (h httpHeaders) String() string {
	var m map[string]string = h
	return fmt.Sprintf("%s", m)
}

func (h httpHeaders) Set(value string) error {
	parts := strings.Split(value, ": ")
	if len(parts) != 2 {
		return fmt.Errorf("header should be of format 'Key: Value'")
	}
	h[parts[0]] = parts[1]
	return nil
}

func main() {
	flag.Usage = Usage
	var host string
	var port int
	var protocol string
	var urlString string
	var framed bool
	var useHttp bool
	headers := make(httpHeaders)
	var parsedUrl *url.URL
	var trans thrift.TTransport
	_ = strconv.Atoi
	_ = math.Abs
	flag.Usage = Usage
	flag.StringVar(&host, "h", "localhost", "Specify host and port")
	flag.IntVar(&port, "p", 9090, "Specify port")
	flag.StringVar(&protocol, "P", "binary", "Specify the protocol (binary, compact, simplejson, json)")
	flag.StringVar(&urlString, "u", "", "Specify the url")
	flag.BoolVar(&framed, "framed", false, "Use framed transport")
	flag.BoolVar(&useHttp, "http", false, "Use http")
	flag.Var(headers, "H", "Headers to set on the http(s) request (e.g. -H \"Key: Value\")")
	flag.Parse()
	
	if len(urlString) > 0 {
		var err error
		parsedUrl, err = url.Parse(urlString)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing URL: ", err)
			flag.Usage()
		}
		host = parsedUrl.Host
		useHttp = len(parsedUrl.Scheme) <= 0 || parsedUrl.Scheme == "http" || parsedUrl.Scheme == "https"
	} else if useHttp {
		_, err := url.Parse(fmt.Sprint("http://", host, ":", port))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error parsing URL: ", err)
			flag.Usage()
		}
	}
	
	cmd := flag.Arg(0)
	var err error
	var cfg *thrift.TConfiguration = nil
	if useHttp {
		trans, err = thrift.NewTHttpClient(parsedUrl.String())
		if len(headers) > 0 {
			httptrans := trans.(*thrift.THttpClient)
			for key, value := range headers {
				httptrans.SetHeader(key, value)
			}
		}
	} else {
		portStr := fmt.Sprint(port)
		if strings.Contains(host, ":") {
			host, portStr, err = net.SplitHostPort(host)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error with host:", err)
				os.Exit(1)
			}
		}
		trans = thrift.NewTSocketConf(net.JoinHostPort(host, portStr), cfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error resolving address:", err)
			os.Exit(1)
		}
		if framed {
			trans = thrift.NewTFramedTransportConf(trans, cfg)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error creating transport", err)
		os.Exit(1)
	}
	defer trans.Close()
	var protocolFactory thrift.TProtocolFactory
	switch protocol {
	case "compact":
		protocolFactory = thrift.NewTCompactProtocolFactoryConf(cfg)
	case "simplejson":
		protocolFactory = thrift.NewTSimpleJSONProtocolFactoryConf(cfg)
	case "json":
		protocolFactory = thrift.NewTJSONProtocolFactory()
	case "binary", "":
		protocolFactory = thrift.NewTBinaryProtocolFactoryConf(cfg)
	default:
		fmt.Fprintln(os.Stderr, "Invalid protocol specified: ", protocol)
		Usage()
		os.Exit(1)
	}
	iprot := protocolFactory.GetProtocol(trans)
	oprot := protocolFactory.GetProtocol(trans)
	client := github.com/veteran-chad/storm-controller/pkg/storm/thrift.NewSupervisorClient(thrift.NewTStandardClient(iprot, oprot))
	if err := trans.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "Error opening socket to ", host, ":", port, " ", err)
		os.Exit(1)
	}
	
	switch cmd {
	case "sendSupervisorAssignments":
		if flag.NArg() - 1 != 1 {
			fmt.Fprintln(os.Stderr, "SendSupervisorAssignments requires 1 args")
			flag.Usage()
		}
		arg782 := flag.Arg(1)
		mbTrans783 := thrift.NewTMemoryBufferLen(len(arg782))
		defer mbTrans783.Close()
		_, err784 := mbTrans783.WriteString(arg782)
		if err784 != nil {
			Usage()
			return
		}
		factory785 := thrift.NewTJSONProtocolFactory()
		jsProt786 := factory785.GetProtocol(mbTrans783)
		argvalue0 := github.com/veteran-chad/storm-controller/pkg/storm/thrift.NewSupervisorAssignments()
		err787 := argvalue0.Read(context.Background(), jsProt786)
		if err787 != nil {
			Usage()
			return
		}
		value0 := argvalue0
		fmt.Print(client.SendSupervisorAssignments(context.Background(), value0))
		fmt.Print("\n")
		break
	case "getLocalAssignmentForStorm":
		if flag.NArg() - 1 != 1 {
			fmt.Fprintln(os.Stderr, "GetLocalAssignmentForStorm requires 1 args")
			flag.Usage()
		}
		argvalue0 := flag.Arg(1)
		value0 := argvalue0
		fmt.Print(client.GetLocalAssignmentForStorm(context.Background(), value0))
		fmt.Print("\n")
		break
	case "sendSupervisorWorkerHeartbeat":
		if flag.NArg() - 1 != 1 {
			fmt.Fprintln(os.Stderr, "SendSupervisorWorkerHeartbeat requires 1 args")
			flag.Usage()
		}
		arg789 := flag.Arg(1)
		mbTrans790 := thrift.NewTMemoryBufferLen(len(arg789))
		defer mbTrans790.Close()
		_, err791 := mbTrans790.WriteString(arg789)
		if err791 != nil {
			Usage()
			return
		}
		factory792 := thrift.NewTJSONProtocolFactory()
		jsProt793 := factory792.GetProtocol(mbTrans790)
		argvalue0 := github.com/veteran-chad/storm-controller/pkg/storm/thrift.NewSupervisorWorkerHeartbeat()
		err794 := argvalue0.Read(context.Background(), jsProt793)
		if err794 != nil {
			Usage()
			return
		}
		value0 := argvalue0
		fmt.Print(client.SendSupervisorWorkerHeartbeat(context.Background(), value0))
		fmt.Print("\n")
		break
	case "":
		Usage()
	default:
		fmt.Fprintln(os.Stderr, "Invalid function ", cmd)
	}
}
