package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	autostartGUI := len(os.Args) == 2 && os.Args[1] == "-autostart"
	if len(os.Args) == 1 || autostartGUI {
		if err := runGUI(autostartGUI); err == nil {
			return
		}
	}

	user := flag.String("u", "", "Login User (Phone Number or Other)")
	password := flag.String("p", "", "Login User Password")
	smsCode := flag.String("c", "", "Pre-enter verification code (optional)")
	showIfaces := flag.Bool("s", false, "Show available network interfaces and exit")
	networkIdx := flag.Int("n", 0, "Network interface number (use -s to list)")
	configPath := flag.String("config", "", "Path to JSON config file")
	logFile := flag.String("log-file", "", "Append logs to this file")
	flag.StringVar(user, "user", "", "Login User (Phone Number or Other)")
	flag.StringVar(password, "password", "", "Login User Password")
	flag.StringVar(smsCode, "sms", "", "Pre-enter verification code (optional)")
	flag.BoolVar(showIfaces, "show", false, "Show available network interfaces and exit")
	flag.IntVar(networkIdx, "network", 0, "Network interface number (use -s to list)")
	flag.Parse()
	providedFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		providedFlags[f.Name] = true
	})

	// -s/--show is exclusive: must not be combined with any other argument
	if *showIfaces {
		if *user != "" || *password != "" || *smsCode != "" || *networkIdx != 0 || *configPath != "" || *logFile != "" {
			fmt.Println("Error: -s/--show cannot be used with other arguments")
			os.Exit(1)
		}
		ifaces, err := ListNetworkInterfaces()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		PrintNetworkInterfaces(ifaces)
		os.Exit(0)
	}

	fileCfg, err := loadFileConfig(*configPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	applyFileConfig(fileCfg, providedFlags, user, password, smsCode, logFile, networkIdx)

	if *user == "" || *password == "" {
		fmt.Println("Usage: esurfinggo -u <user> -p <password> [-c <sms_code>] [-n <interface_number>] [-log-file <file>]")
		fmt.Println("       esurfinggo -config <file> [-log-file <file>]")
		fmt.Println("       esurfinggo -s")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Validate -n/--network if specified
	var selectedIface *NetworkInterface
	if *networkIdx != 0 {
		ifaces, err := ListNetworkInterfaces()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		selectedIface, err = GetNetworkInterfaceByIndex(ifaces, *networkIdx)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Use -s/--show to list available interfaces")
			os.Exit(1)
		}
	}

	opts := Options{
		LoginUser:     *user,
		LoginPassword: *password,
		SMSCode:       *smsCode,
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	logHandle, err := setupLogOutput(*logFile)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	if logHandle != nil {
		defer logHandle.Close()
	}
	log.Printf("[Main] Starting ESurfing Go client")
	log.Printf("[Main] User supplied (%s)", previewForLog(*user))

	// Create bound transport if a specific interface is selected
	var boundTransport *http.Transport
	if selectedIface != nil {
		var err error
		boundTransport, err = NewBoundHTTPTransport(selectedIface)
		if err != nil {
			fmt.Printf("Error: failed to bind to interface %s: %v\n", selectedIface.Name, err)
			os.Exit(1)
		}
		log.Printf("[Main] Using network interface: %s (#%d)", selectedIface.Name, selectedIface.Index)
	} else if transport, interfaceName, err := NewTUNSafeHTTPTransport(); err != nil {
		log.Printf("[Main] TUN-safe transport unavailable: %v; using system routing", err)
	} else if transport != nil {
		boundTransport = transport
		log.Printf("[Main] TUN detected; binding authentication traffic to physical interface: %s", interfaceName)
	}

	states := NewStates()
	session := NewSession()

	var client *Client
	if boundTransport != nil {
		client = NewClient(opts, states, session, boundTransport)
	} else {
		client = NewClient(opts, states, session)
	}

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if states.IsRunning() {
			states.SetRunning(false)
		}
		if session.IsInitialized() {
			if states.IsLogged() {
				client.Term()
			}
			session.Free()
		}
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	states.RefreshStates()
	log.Printf("[Main] Client-ID: %s", states.GetClientID())
	log.Printf("[Main] MAC: %s", states.GetMacAddress())
	client.Run()
}
