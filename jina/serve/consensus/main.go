package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
	"path/filepath"
	"C"

	"github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
	transport "github.com/Jille/raft-grpc-transport"
	"github.com/Jille/raftadmin"
	"github.com/hashicorp/raft"
	boltdb "github.com/hashicorp/raft-boltdb"
	jinaraft "jina-raft/jina_raft"
	pb "jina-raft/jina-go-proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func NewRaft(ctx context.Context, myID, myAddress string, raftDir string, raftBootstrap bool, fsm raft.FSM) (*raft.Raft, *transport.Manager, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(myID)
	config.SnapshotThreshold = 3
	config.SnapshotInterval = 10 * time.Second

	baseDir := filepath.Join(raftDir, myID)

	logs_db, err := boltdb.NewBoltStore(filepath.Join(baseDir, "logs.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "logs.dat"), err)
	}

	stable_db, err := boltdb.NewBoltStore(filepath.Join(baseDir, "stable.dat"))
	if err != nil {
		return nil, nil, fmt.Errorf(`boltdb.NewBoltStore(%q): %v`, filepath.Join(baseDir, "stable.dat"), err)
	}

	file_snapshot, err := raft.NewFileSnapshotStore(baseDir, 3, os.Stderr)
	if err != nil {
		return nil, nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q, ...): %v`, baseDir, err)
	}

	tm := transport.New(raft.ServerAddress(myAddress), []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())})

	r, err := raft.NewRaft(config, fsm, logs_db, stable_db, file_snapshot, tm.Transport())
	if err != nil {
		return nil, nil, fmt.Errorf("raft.NewRaft: %v", err)
	}

	if raftBootstrap {
		cfg := raft.Configuration{
			Servers: []raft.Server{
				{
					Suffrage: raft.Voter,
					ID:       raft.ServerID(myID),
					Address:  raft.ServerAddress(myAddress),
				},
			},
		}
		f := r.BootstrapCluster(cfg)
		if err := f.Error(); err != nil {
			return nil, nil, fmt.Errorf("raft.Raft.BootstrapCluster: %v", err)
		}
	}

	return r, tm, nil
}

//export RunC
func RunC(myAddr *C.char, raftId *C.char, raftDir *C.char, raftBootstrap bool, executorTarget *C.char) {
    Run(C.GoString(myAddr), C.GoString(raftId), C.GoString(raftDir), raftBootstrap, C.GoString(executorTarget))
}

func Run(myAddr string, raftId string, raftDir string, raftBootstrap bool, executorTarget string) {
    log.Printf("Calling Run %s, %s, %s, %s", myAddr, raftId, raftDir, executorTarget)
    if raftId == "" {
        log.Fatalf("flag --raft_id is required")
    }

    ctx := context.Background()
    _, port, err := net.SplitHostPort(myAddr)
    if err != nil {
        log.Fatalf("failed to parse local address (%q): %v", myAddr, err)
    }
    sock, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    executorFSM := jinaraft.NewExecutorFSM(executorTarget)

    raft, tm, err := NewRaft(ctx, raftId, myAddr, raftDir, raftBootstrap, executorFSM)
    if err != nil {
        log.Fatalf("failed to start raft: %v", err)
    }
    grpcServer := grpc.NewServer()
    pb.RegisterJinaSingleDataRequestRPCServer(grpcServer, &jinaraft.RpcInterface{
        Executor: executorFSM,
        Raft:     raft,
    })
    tm.Register(grpcServer)
    leaderhealth.Setup(raft, grpcServer, []string{"Health"})
    raftadmin.Register(grpcServer, raft)
    reflection.Register(grpcServer)
    if err := grpcServer.Serve(sock); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}

func main() {
    myAddr         := flag.String("address", "localhost:50051", "TCP host+port for this node")
    raftId         := flag.String("raft_id", "", "Node id used by Raft")
    raftDir        := flag.String("raft_data_dir", "data/", "Raft data dir")
    raftBootstrap  := flag.Bool("raft_bootstrap", false, "Whether to bootstrap the Raft cluster")
    executorTarget := flag.String("executor_target", "localhost:54321", "underlying executor host+port")
	flag.Parse()
	log.Printf("Calling main")
    Run(*myAddr, *raftId, *raftDir, *raftBootstrap, *executorTarget)
}

