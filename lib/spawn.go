package rtmkt

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-multistore"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/filecoin-project/lotus/api"
	lclient "github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// NodeType is the role our node can play in the retrieval market
type NodeType uint64

const (
	// NodeTypeClient nodes can only retrieve content
	NodeTypeClient NodeType = iota
	// NodeTypeProvider nodes can only provide content
	NodeTypeProvider
	// NodeTypeFull nodes can do everything
	// mostly TODO here
	NodeTypeFull
)

type MyelNode struct {
	Ctx      context.Context
	Store    *ipfsStore
	Gossip   *Gossip
	Client   RetrievalClient
	Provider RetrievalProvider
	Wallet   *wallet.LocalWallet
	PaychMgr *paychManager
	Lotus    api.FullNode
	lcloser  jsonrpc.ClientCloser
}

func (mn *MyelNode) Close() {
	mn.lcloser()
	mn.Store.node.Close()
}

func SpawnNode(nt NodeType) (*MyelNode, error) {
	ctx := context.Background()

	tok := flag.String("auth_token", "", "authorization token for infura lotus service")
	flag.Parse()

	etok := base64.StdEncoding.EncodeToString([]byte(*tok))

	// Establish connection with a remote (or local) lotus node
	lapi, lcloser, err := lclient.NewFullNodeRPC(ctx, "wss://filecoin.infura.io", http.Header{
		// Provide a write token if you're running your own lotus
		"Authorization": []string{fmt.Sprintf("Basic %s", etok)},
	})
	if err != nil {
		return nil, fmt.Errorf("Unable to start lotus rpc: %v", err)
	}
	// Create an underlying ipfs node
	ipfs, err := NewIpfsStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("Unable to start ipfs node: %v", err)
	}

	// Create a gossipsub network for cluster communications
	g, err := NewGossip(ctx, ipfs.node.PeerHost)
	if err != nil {
		return nil, fmt.Errorf("Unable to start gossipsub: %v", err)
	}

	// Create a retrieval network protocol from the ipfs node libp2p host
	net := NewFromLibp2pHost(ipfs.node.PeerHost)
	// Get the Datastore from ipfs
	ds := ipfs.node.Repo.Datastore()
	memks := wallet.NewMemKeyStore()
	w, err := wallet.NewWallet(memks)
	if err != nil {
		return nil, fmt.Errorf("Unable to create new wallet: %v", err)
	}
	actStore := cbor.NewCborStore(ipfs.node.Blockstore)
	paychMgr := NewPaychManager(ctx, lapi, w, ds, actStore)

	// Wrap the full node api to provide an adapted interface
	radapter := NewRetrievalNode(lapi, paychMgr, TestModeOff)

	node := &MyelNode{
		Ctx:      ctx,
		Store:    ipfs,
		Gossip:   g,
		Wallet:   w,
		PaychMgr: paychMgr,
		Lotus:    lapi,
		lcloser:  lcloser,
	}
	if nt == NodeTypeClient || nt == NodeTypeFull {
		// Create a new namespace for our metadata store
		nds := namespace.Wrap(ds, datastore.NewKey("/retrieval/client"))

		multiDs, err := multistore.NewMultiDstore(ds)
		if err != nil {
			return nil, fmt.Errorf("Unable to create multistore: %v", err)
		}
		dataTransfer, err := NewDataTransfer(ctx, ipfs.node, nds)
		if err != nil {
			return nil, fmt.Errorf("Unable to create graphsync data transfer: %v", err)
		}
		err = dataTransfer.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("Unable to start data transfer: %v", err)
		}

		if _, err := node.WalletImport("client.private"); err != nil {
			return nil, err
		}
		countKey := datastore.NewKey("/retrieval/client/dealcounter")
		dealCounter := storedcounter.New(ds, countKey)
		resolver := NewLocalPeerResolver(ds)

		client, err := NewClient(radapter, net, multiDs, dataTransfer, nds, resolver, dealCounter)
		if err != nil {
			return nil, fmt.Errorf("Unable to create new retrieval client: %v", err)
		}
		node.Client = client

		err = client.Start(ctx)
		if err != nil {
			return nil, err
		}
	}
	if nt == NodeTypeProvider || nt == NodeTypeFull {
		pds := namespace.Wrap(ds, datastore.NewKey("/retrieval/provider"))

		multiDs, err := multistore.NewMultiDstore(pds)
		if err != nil {
			return nil, fmt.Errorf("Unable to create multistore: %v", err)
		}
		dataTransfer, err := NewDataTransfer(ctx, ipfs.node, pds)
		if err != nil {
			return nil, fmt.Errorf("Unable to create graphsync data transfer: %v", err)
		}
		err = dataTransfer.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("Unable to start data transfer: %v", err)
		}

		providerAddress, err := node.WalletImport("provider.private")
		if err != nil {
			return nil, err
		}

		provider, err := NewProvider(providerAddress, radapter, net, multiDs, ipfs, dataTransfer, pds)
		if err != nil {
			return nil, fmt.Errorf("Unable to create new retrieval provider: %v", err)
		}
		node.Provider = provider

		err = provider.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("Unable to start provider: %v", err)
		}
	}

	return node, nil
}

func (mn *MyelNode) WalletImport(name string) (address.Address, error) {
	wd, err := os.Getwd()
	if err != nil {
		return address.Undef, fmt.Errorf("Unable to get current directory: %v", err)
	}
	fdata, err := ioutil.ReadFile(path.Join(wd, name))
	// If no key is available for import we create a default key
	if err != nil {
		err = nil
		addr, err := mn.Wallet.WalletNew(mn.Ctx, types.KTBLS)
		if err != nil {
			return address.Undef, fmt.Errorf("Unable generate key: %v", err)
		}
		if err = mn.Wallet.SetDefault(addr); err != nil {
			return address.Undef, err
		}
		return addr, nil
	}
	var ki types.KeyInfo
	data, err := hex.DecodeString(strings.TrimSpace(string(fdata)))
	if err != nil {
		return address.Undef, fmt.Errorf("Unable to decode hex string: %v", err)
	}
	if err := json.Unmarshal(data, &ki); err != nil {
		return address.Undef, fmt.Errorf("Unable to unmarshal keyinfo: %v", err)
	}
	addr, err := mn.Wallet.WalletImport(mn.Ctx, &ki)
	if err := mn.Wallet.SetDefault(addr); err != nil {
		return address.Undef, err
	}
	return addr, nil
}
