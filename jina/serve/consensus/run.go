package main

// #include <Python.h>
// #include <stdbool.h>
// int PyArg_ParseTuple_run(PyObject * args, char **a, char **b, char **c, bool *d, char **e);
// int PyArg_ParseTuple_add_voter(PyObject * args, char **a, char **b, char **c);
// void raise_exception(char *msg);
import "C"


import (
    "context"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"
    "path/filepath"

    "github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
    transport "github.com/Jille/raft-grpc-transport"
    "github.com/Jille/raftadmin"
    "github.com/hashicorp/raft"
    boltdb "github.com/hashicorp/raft-boltdb"
    jinaraft "jraft/jina_raft"
    pb "jraft/jina-go-proto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/reflection"
)

func NewRaft(ctx context.Context, myID, myAddress string, raftDir string, raftBootstrap bool, fsm raft.FSM) (*raft.Raft, *transport.Manager, error) {

    config := raft.DefaultConfig()
    config.LocalID = raft.ServerID(myID)

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

func Run(myAddr string, raftId string, raftDir string, raftBootstrap bool, executorTarget string) {
    log.Printf("Calling Run %s, %s, %s, %p, %s", myAddr, raftId, raftDir, raftBootstrap, executorTarget)
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

    r, tm, err := NewRaft(ctx, raftId, myAddr, raftDir, raftBootstrap, executorFSM)
    if err != nil {
        log.Fatalf("failed to start raft: %v", err)
    }
    grpcServer := grpc.NewServer()
    pb.RegisterJinaSingleDataRequestRPCServer(grpcServer, &jinaraft.RpcInterface{
        Executor: executorFSM,
        Raft:     r,
    })
    pb.RegisterJinaDiscoverEndpointsRPCServer(grpcServer, &jinaraft.RpcInterface{
        Executor: executorFSM,
        Raft:     r,
    })
    pb.RegisterJinaInfoRPCServer(grpcServer, &jinaraft.RpcInterface{
        Executor: executorFSM,
        Raft:     r,
    })
    tm.Register(grpcServer)
    leaderhealth.Setup(r, grpcServer, []string{"Health"})

    raftadmin.Register(grpcServer, r)
    reflection.Register(grpcServer)
    sigchnl := make(chan os.Signal, 1)
    signal.Notify(sigchnl, syscall.SIGINT, syscall.SIGTERM)
    go func(){
        sig := <-sigchnl
        log.Printf("Signal %v received", sig)
        grpcServer.Stop()
        shutdownResult := r.Shutdown()
        err := shutdownResult.Error()
        if err != nil {
            log.Fatalf("Error returned while shutting RAFT down: %v", err)
        }
        os.Exit(0)
    }()
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
    Run(*myAddr, *raftId, *raftDir, *raftBootstrap, *executorTarget)
}


//export run
func run(self *C.PyObject, args *C.PyObject) *C.PyObject {
    var myAddr *C.char
    var raftId *C.char
    var raftDir *C.char
    var raftBootstrap C.bool
    var executorTarget *C.char
    if C.PyArg_ParseTuple_run(args, &myAddr, &raftId, &raftDir, &raftBootstrap, &executorTarget) != 0 {
        Run(C.GoString(myAddr), C.GoString(raftId), C.GoString(raftDir), raftBootstrap != false, C.GoString(executorTarget))
    }
    C.Py_IncRef(C.Py_None);
    return C.Py_None;
}

//export add_voter
func add_voter(self *C.PyObject, args *C.PyObject) *C.PyObject {
    var target *C.char
    var raftId *C.char
    var voterAddress *C.char
    if C.PyArg_ParseTuple_add_voter(args, &target, &raftId, &voterAddress) != 0 {
        err := AddVoter(C.GoString(target), C.GoString(raftId), C.GoString(voterAddress))
        if err != nil {
            log.Printf("Error received calling AddVoter %v, but return None", err)
            C.raise_exception(C.CString("Error from AddVoter"))
            return nil
        }
    }
    C.Py_IncRef(C.Py_None);
    return C.Py_None;
}
