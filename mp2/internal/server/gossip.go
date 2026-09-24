package server

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	pb "cs425/proto/membership"
)


func (s *Server) GossipJoin(ctx context.Context, req *pb.GossipJoinRequest) (*pb.GossipJoinResponse, error) {
	if !s.introducer {
		return nil, fmt.Errorf("server is not the introducer")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	newNodeID := req.NodeId
	s.membershipList[newNodeID] = &pb.NodeState{
		Heartbeat:   0,
		Incarnation: 0,
		Status:      "Alive",
	}
	s.localTimeList[newNodeID] = time.Now()
	snapshot := make(map[string]*pb.NodeState, len(s.membershipList))
	for k, v := range s.membershipList {
		snapshot[k] = cloneNodeState(v)
	}
	fmt.Println("New node joined cluster", "nodeID", newNodeID)
	s.log.Info("New node joined cluster", "nodeID", newNodeID)
	return &pb.GossipJoinResponse{
		Membership: snapshot,
	}, nil

}

func (s *Server) GossipPush(ctx context.Context, req *pb.GossipPushRequest) (*pb.GossipPushResponse, error) {
	if rand.Float64() < s.dropRate {
		return &pb.GossipPushResponse{}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for nodeID, remoteState := range req.Membership {
		if remoteState == nil {
			continue
		}
		localState, exists := s.membershipList[nodeID]
		if exists && localState != nil && localState.Status == "Failed" {
			continue
		}

		if !exists || localState == nil{
			if s.tombstones[nodeID] {
				continue
			}
			s.membershipList[nodeID] = cloneNodeState(remoteState)
			s.localTimeList[nodeID] = time.Now()
			continue
		}
		if remoteState.Status == "Failed" {
			localState.Status = "Failed"
			s.localTimeList[nodeID] = time.Now()
			continue
		}
		if s.enableSuspicion {
			if remoteState.Incarnation > localState.Incarnation {
				s.membershipList[nodeID] = cloneNodeState(remoteState)
				s.localTimeList[nodeID] = time.Now()
			} else if remoteState.Incarnation == localState.Incarnation {
				updated := false
				if remoteState.Status == localState.Status {
					if remoteState.Heartbeat > localState.Heartbeat {
						updated = true
					}
				} else {
					if localState.Status == "Alive" && remoteState.Status == "Suspect" {
						updated = true
					}
				}
				if updated {
					s.membershipList[nodeID] = cloneNodeState(remoteState)
					s.localTimeList[nodeID] = time.Now()
				}
			}
		} else {
			if remoteState.Heartbeat > localState.Heartbeat {
				s.membershipList[nodeID] = cloneNodeState(remoteState)
				s.localTimeList[nodeID] = time.Now()
			}
		}
	}
	return &pb.GossipPushResponse{}, nil
}