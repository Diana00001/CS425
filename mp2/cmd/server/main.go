package main

import (
	"cs425/internal/server"
	"cs425/pkg/logger"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"bufio"
	"time"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	id := flag.Int("id", 1, "server ID")
	host := flag.String("host", "localhost", "server host")
	port := flag.Int("port", 8080, "server port")
	// TODO: Add Flag for introducer
	// Make it false by default
	isIntroducer := flag.Bool("introducer", false, "is this node the introducer")
	introducerHost := flag.String("intro-host", "localhost", "introducer host")
	introducerPort := flag.Int("intro-port", 8080, "introducer port")
	enableSuspicion := flag.Bool("suspect", false, "enable suspicion mechanism by default")

	flag.Parse()


	path := filepath.Join(
		"logs",
		fmt.Sprintf("machine.%d.log", *id),
	)

	serverLog, logFile, err := logger.Open(path)
	if err != nil {
		log.Printf("initialize server logging: %v", err)
		return err
	}
	defer logFile.Close()

	serverLog = serverLog.With("machine", *id)

	s, err := server.NewServer(*id, *host, *port, serverLog, *isIntroducer)
	if err != nil {
		serverLog.Error("create server", "err", err)
		return err
	}
	s.SwitchProtocol(*enableSuspicion)
	go func() {
		if err := s.ListenAndServe(); err != nil {
			serverLog.Error("server stopped", "err", err)
		}
	}()

	if err := s.Heartbeat(); err != nil {
		serverLog.Error("start heartbeat failed", "err", err)
		return err
	}

	if err := s.Disseminate(); err != nil {
		serverLog.Error("start disseminate failed", "err", err)
		return err
	}

	// failTimeout (3s), suspectTimeout (3s), cleanupTimeout (6s)
	s.StartTimeoutChecker(3*time.Second, 3*time.Second, 6*time.Second)

	if !*isIntroducer {
		serverLog.Info("Joining cluster via introducer...", "introHost", *introducerHost, "introPort", *introducerPort)
		if err := s.Join(*introducerHost, *introducerPort); err != nil {
			serverLog.Error("failed to join cluster", "err", err)
			return err
		}
	} else {
		serverLog.Info("Server started as the cluster Introducer")
	}

	go handleCLI(s, *introducerHost, *introducerPort)
	select {}

	return nil
}

func handleCLI(s *server.Server, defaultIntroHost string, defaultIntroPort int) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "list_mem":
			s.PrintMembership()

		case "list_self":
			nodeIDStr, _ := s.NodeID()
			fmt.Println("My NodeID:", nodeIDStr)

		case "join":
			// join [intro_host] [intro_port]
			introHost := defaultIntroHost
			introPort := defaultIntroPort
			if len(parts) > 1 {
				introHost = parts[1]
			}
			if len(parts) > 2 {
				_, _ = fmt.Sscanf(parts[2], "%d", &introPort)
			}

			fmt.Printf("Manually joining cluster via introducer at %s:%d...\n", introHost, introPort)
			if err := s.Join(introHost, introPort); err != nil {
				fmt.Printf("Join failed: %v\n", err)
			} else {
				fmt.Println("-> Successfully joined the cluster!")
			}

		case "leave":
			fmt.Println("Leaving group...")
			_ = s.Leave()
			time.Sleep(300 * time.Millisecond)
			os.Exit(0)

		case "display_suspects":
			s.PrintSuspendHistory()

		case "switch":
			if len(parts) > 1 {
				if parts[1] == "suspect" {
					s.SwitchProtocol(true)
					fmt.Println("Suspicion enabled.")
				} else if parts[1] == "nosuspect" {
					s.SwitchProtocol(false)
					fmt.Println("Suspicion disabled.")
				}
			}

		case "display_protocol":
			s.PrintProtocolStatus()

		case "set_drop_rate":
			if len(parts) > 1 {
				var rate float64
				_, _ = fmt.Sscanf(parts[1], "%f", &rate)
				s.SetDropRate(rate)
				fmt.Printf("Drop rate set to %.2f%%\n", rate*100)
			}

		default:
			fmt.Println("Unknown command. Supported: list_mem, list_self, leave, display_suspects, switch {suspect, nosuspect}, display_protocol, set_drop_rate {rate}")
		}
	}
}