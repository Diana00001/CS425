package server

import (
	// pb "cs425/proto/membership"
	
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"context"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "cs425/proto/membership"
)

const maxMessageSize = 256 * 1024 * 1024

type SuspectRecord struct {
	NodeID    string
	SuspectAt time.Time
}

type Server struct {
	pb.UnimplementedMembershipServiceServer

	mu   sync.RWMutex
	id   int
	host string
	port int
	timestamp int64
	log  *slog.Logger

	heartbeat       int32
	incarnation     int32
	introducer      bool
	dropRate        float64
	enableSuspicion bool

	// membershipList  map[string]int // NodeID -> Heartbeat
	membershipList map[string]*pb.NodeState // NodeID -> NodeState
	// suspicionList   map[string]int
	// suspensionList map[string]string
	// statusList map[string]string   // NodeID -> Status (Alive, Suspect, Failed)
	// incarnationList map[string]int // NodeID -> Incarnation Count
	// localTimeList   map[string]int
	localTimeList  map[string]time.Time // NodeID -> Last Updated Timestamp
	suspectHistory  []SuspectRecord
}

func NewServer(id int, host string, port int, log *slog.Logger, introducer bool) (*Server, error) {
	// Pass introducer into validator
	if err := validateNewServer(id, host, port, log); err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	selfID := fmt.Sprintf("%d:%s:%d:%d", id, host, port, timestamp)
	membershipList := map[string]*pb.NodeState{
		selfID: {Heartbeat: 0, Incarnation: 0, Status: "Alive"},
	}
	localTimeList := map[string]time.Time{selfID: time.Now()}

	return &Server{
		id:          id,
		timestamp:   timestamp,
		host:        host,
		port:        port,
		log:         log,
		heartbeat:   0,
		incarnation: 0,
		introducer:  introducer,
		dropRate:    0.0,
		enableSuspicion: false,

		// membershipList:    make(map[string]int),
		membershipList:    membershipList,
		// suspensionList:    make(map[string]int),
		// suspensionList: make(map[string]string),
		// incarnationList:   make(map[string]int),
		//localTimeList:     make(map[string]int),
		localTimeList:  localTimeList,

	}, nil
}

func (s *Server) ListenAndServe() error {
	// TODO: Implement
	ln, err := net.Listen("tcp", s.address())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address(), err)
	}
	defer ln.Close()

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
	)

	pb.RegisterMembershipServiceServer(grpcServer, s)

	s.log.Info("server listening", "addr", s.address())

	if err := grpcServer.Serve(ln); err != nil {
		return fmt.Errorf("serve gRPC: %w", err)
	}

	return nil
}

func (s *Server) address() string {
	return net.JoinHostPort(s.host, strconv.Itoa(s.port))
}

func validateNewServer(id int, host string, port int, log *slog.Logger) error {
	if id < 1 {
		return fmt.Errorf("invalid server ID")
	}

	if host == "" {
		return fmt.Errorf("invalid server host")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid server port")
	}

	if log == nil {
		return fmt.Errorf("invalid server logger")
	}

	// TODO: Add introducer validation

	return nil
}

func (s *Server) NodeID() (string, error) {
	// TODO: Implement
	if s.host == "" || s.port <= 0 || s.id <= 0 {
		return "", fmt.Errorf("server not fully initialized")
	}

	return fmt.Sprintf("%d:%s:%d:%d", s.id, s.host, s.port, s.timestamp), nil
}

func parseAddressFromNodeID(nodeID string) string {
	parts := strings.Split(nodeID, ":")
	if len(parts) >= 4 {
		return net.JoinHostPort(parts[1], parts[2])
	}
	return nodeID
}

// deep copy of NodeState
func cloneNodeState(v *pb.NodeState) *pb.NodeState {
	if v == nil {
		return nil
	}
	return &pb.NodeState{
		Heartbeat:   v.Heartbeat,
		Incarnation: v.Incarnation,
		Status:      v.Status,
	}
}

func (s *Server) SetDropRate(rate float64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.dropRate = rate
    s.log.Info("Drop rate updated", "dropRate", rate)
}

func (s *Server) Heartbeat() error {
	// TODO: Implement
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			s.mu.Lock()
			s.heartbeat++
			selfID, err := s.NodeID()
			if err != nil {
				s.log.Error("Failed to get node ID", "err", err)
				s.mu.Unlock()
				continue
			}
			state, ok := s.membershipList[selfID]
			if !ok {
				state = &pb.NodeState{
					Heartbeat:   s.heartbeat,
					Incarnation: s.incarnation,
					Status:      "Alive",
				}
				s.membershipList[selfID] = state
			} else {
				state.Heartbeat = s.heartbeat
				if s.enableSuspicion && state.Status == "Suspect" {
					s.incarnation++
					state.Incarnation = s.incarnation
					state.Status = "Alive"
				}
			}
			s.localTimeList[selfID] = time.Now()
			s.mu.Unlock()
		}
	}()
	return nil
}

func (s *Server) Disseminate() error {
	// TODO: Implement
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			s.Push()
		}
	}()
	return nil
}

func (s *Server) Push() error {
	// TODO: Implement

	s.mu.RLock()
	selfID, err := s.NodeID()
	if err != nil {
		s.log.Error("Failed to get node ID", "err", err)
		return fmt.Errorf("failed to get node ID: %w", err)
	}
	peers := make([]string, 0)
	for nodeID, state := range s.membershipList {
		if nodeID != selfID && state != nil && state.Status != "Failed" {
			peers = append(peers, nodeID)
		}
	}

	payload := make(map[string]*pb.NodeState, len(s.membershipList))
	for k, v := range s.membershipList {
		payload[k] = cloneNodeState(v)
	}
	s.mu.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	// random select 3 nodes
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	targetCount := 3
	if len(peers) < targetCount {
		targetCount = len(peers)
	}

	for i := 0; i < targetCount; i++ {
		targetAddr := parseAddressFromNodeID(peers[i])

		go func(addr string, payload map[string]*pb.NodeState) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
			if err != nil {
				return
			}
			defer conn.Close()

			client := pb.NewMembershipServiceClient(conn)
			_, _ = client.GossipPush(ctx, &pb.GossipPushRequest{
				Membership: payload,
			})
		}(targetAddr, payload)
	}
	return nil

}



func (s *Server) Join(introducerHost string, introducerPort int) error {
	s.mu.Lock()
	selfID, err := s.NodeID()
	if err != nil {
		s.log.Error("Failed to get node ID", "err", err)
		return fmt.Errorf("failed to get node ID: %w", err)
	}
	s.membershipList[selfID] = &pb.NodeState{
		Heartbeat:   s.heartbeat,
		Incarnation: s.incarnation,
		Status:      "Alive",
	}
	s.localTimeList[selfID] = time.Now()
	s.mu.Unlock()

	if s.introducer {
		s.log.Info("server joined as introducer", "nodeID", selfID)
		return nil
	}

	s.log.Info("server joining cluster via introducer...")

	introducerAddr := net.JoinHostPort(introducerHost, strconv.Itoa(introducerPort))

	var conn *grpc.ClientConn
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		conn, err = grpc.DialContext(ctx, introducerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		cancel()
		if err == nil {
			break
		}
		s.log.Info("Retrying connection to introducer...", "attempt", i+1, "addr", introducerAddr)
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to introducer at %s after retries: %w", introducerAddr, err)
	}

	defer conn.Close()

	client := pb.NewMembershipServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := client.GossipJoin(ctx, &pb.GossipJoinRequest{
		NodeId: selfID,
	})
	if err != nil {
		return fmt.Errorf("gossip join RPC failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for nodeID, remoteState := range resp.Membership {
		if _, exists := s.membershipList[nodeID]; !exists {
			s.membershipList[nodeID] = cloneNodeState(remoteState)
			s.localTimeList[nodeID] = time.Now()
		}
	}

	s.log.Info("Successfully joined cluster via introducer", "totalMembers", len(s.membershipList))
	fmt.Println("Successfully joined cluster via introducer", "totalMembers", len(s.membershipList))
	return nil
}



func (s *Server) StartTimeoutChecker(failTimeout, suspectTimeout, cleanupTimeout time.Duration) {
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		for range ticker.C {
			s.mu.Lock()
			selfID, err := s.NodeID()
			if err != nil {
				s.log.Error("Failed to get node ID", "err", err)
				s.mu.Unlock()
				continue
			}
			now := time.Now()

			for nodeID, lastTime := range s.localTimeList {
				if nodeID == selfID {
					continue
				}

				state, exists := s.membershipList[nodeID]
				if !exists || state == nil {
					continue
				}

				if state.Status == "Alive" && now.Sub(lastTime) > failTimeout {
					if s.enableSuspicion {
						state.Status = "Suspect"
						s.suspectHistory = append(s.suspectHistory, SuspectRecord{NodeID: nodeID, SuspectAt: now})
						s.log.Info("Node marked as Suspect", "nodeID", nodeID)
						fmt.Println("Node marked as Suspect", "nodeID", nodeID)
					} else {
						state.Status = "Failed"
						s.log.Info("Node marked as Failed due to timeout", "nodeID", nodeID)
						fmt.Println("Node marked as Failed due to timeout", "nodeID", nodeID)
					}
					s.localTimeList[nodeID] = now
					} else if s.enableSuspicion && state.Status == "Suspect" && now.Sub(lastTime) > suspectTimeout {
						state.Status = "Failed"
						s.log.Info("Suspect node confirmed Failed (suspicion timeout expired)", "nodeID", nodeID)
						s.localTimeList[nodeID] = now
						fmt.Println("Suspect node confirmed Failed (suspicion timeout expired)", "nodeID", nodeID)
					}

				if state.Status == "Failed" && now.Sub(lastTime) > cleanupTimeout {
					delete(s.membershipList, nodeID)
					delete(s.localTimeList, nodeID)
					s.log.Info("Node cleaned up from memory", "nodeID", nodeID)
					fmt.Println("Node cleaned up from memory", "nodeID", nodeID)
				}
			}
			s.mu.Unlock()
		}
	}()
}

func (s *Server) Leave() error {
	s.mu.Lock()
	selfID, err := s.NodeID()
	if err != nil {
		s.log.Error("Failed to get node ID", "err", err)
		return fmt.Errorf("failed to get node ID: %w", err)
	}
	s.membershipList[selfID].Status = "Failed"
	s.localTimeList[selfID] = time.Now()
	s.mu.Unlock()
	s.Push()
	s.log.Info("server successfully left the cluster")
	fmt.Println("server successfully left the cluster")
	return nil
}
func (s *Server) PrintSuspendHistory() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Println("Suspect History: ")
	if len(s.suspectHistory) == 0 {
		fmt.Println("No nodes have been suspected yet.")
		return
	}
	for _, record := range s.suspectHistory {
		fmt.Printf("Node: %s | Suspected At: %s\n", record.NodeID, record.SuspectAt.Format("15:04:05.000"))
	}
	fmt.Println("End of Suspect History")
}

func (s *Server) PrintProtocolStatus() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := "Pure Gossip"
	if s.enableSuspicion {
		status := "Gossip + Suspicion"
		fmt.Printf("Current Protocol: %s\n", status)
		return
	}
	fmt.Printf("Current Protocol: %s\n", status)
}

func (s *Server) SwitchProtocol(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enableSuspicion = enabled
	s.log.Info("Protocol mode changed", "enableSuspicion", enabled)
}

func (s *Server) GetNodeIDStr() string {
    idStr, err := s.NodeID()
    if err != nil {
        return "unknown"
    }
    return idStr
}

func (s *Server) PrintMembership() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fmt.Printf("Node %s Membership List (Suspicion=%v): \n", s.GetNodeIDStr(), s.enableSuspicion)
	for id, state := range s.membershipList {
		if state == nil {
			continue
		}
		fmt.Printf("  Node: %s | Heartbeat: %d | Incarnation: %d | Status: %s\n",
			id, state.Heartbeat, state.Incarnation, state.Status)
	}
	fmt.Println("End of Membership List")
}