package braft

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/proto"
	"github.com/bingoohuang/braft/util"
	"google.golang.org/grpc"
)

// ApplyOnLeader apply a payload on the leader node.
func (n *Node) ApplyOnLeader(payload []byte) (interface{}, error) {
	addr := string(n.Raft.Leader())
	if addr == "" {
		return nil, errors.New("unknown leader")
	}

	addr = strings.Replace(addr, HostZero, "127.0.0.1", 1)
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := proto.NewRaftClient(conn).ApplyLog(context.Background(), &proto.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	result, err := n.conf.Serializer.Deserialize(response.Response)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetPeerDetails returns the remote peer details.
func GetPeerDetails(address string) (*proto.GetDetailsResponse, error) {
	address = strings.Replace(address, HostZero, "127.0.0.1", 1)
	c, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer c.Close()

	response, err := proto.NewRaftClient(c).GetDetails(context.Background(), &proto.GetDetailsRequest{})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// HostZero is for the all zeros host.
const HostZero = "0.0.0.0"

var (
	// EnvRport is the raft cluster internal port.
	EnvRport = util.GetEnvInt("BRAFT_RPORT", util.FindFreePort(15000-2))
	// EnvDport is for the raft cluster nodes discovery.
	EnvDport = util.FindFreePort(util.GetEnvInt("BRAFT_DPORT", EnvRport+1))
	// EnvHport is used for the http service.
	EnvHport = util.FindFreePort(util.GetEnvInt("BRAFT_HPORT", EnvDport+1))
	// EnvDiscovery is the discovery method for the raft cluster.
	// e.g.
	// static:192.168.1.1:1500,192.168.1.2:1500,192.168.1.3:1500
	// k8s:svcType=braft;svcBiz=rig
	// mdns:_braft._tcp
	EnvDiscovery = func() discovery.Discovery {
		env := os.Getenv("BRAFT_DISCOVERY")
		s := strings.ToLower(os.Getenv("BRAFT_DISCOVERY"))
		switch {
		case s == "k8s" || strings.HasPrefix(s, "k8s:"):
			var serviceLabels map[string]string
			if strings.HasPrefix(s, "k8s:") {
				s1 := strings.TrimPrefix(env, "k8s:")
				serviceLabels = util.ParseStringToMap(s1, ";", ":")
			}
			return discovery.NewKubernetesDiscovery("", serviceLabels, "")
		case s == "mdns" || s == "" || strings.HasPrefix(s, "mdns:"):
			if strings.HasPrefix(s, "mdns:") {
				s = strings.TrimPrefix(s, "mdns:")
			} else {
				s = ""
			}
			return discovery.NewMdnsDiscovery(s)
		default:
			s = strings.TrimPrefix(s, "static:")
			return discovery.NewStaticDiscovery(strings.Split(s, ","))
		}
	}()
)
