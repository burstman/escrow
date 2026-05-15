package fabric

import (
	"crypto"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"escrow/internal/config"
)

func loadCertificate(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return identity.CertificateFromPEM(data)
}

func loadPrivateKey(path string) (crypto.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return identity.PrivateKeyFromPEM(data)
}

type Client struct {
	contract *client.Contract
	devMode  bool
}

func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.DevMode {
		return &Client{devMode: true}, nil
	}

	cert, err := loadCertificate(cfg.FabricCertPath)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}
	key, err := loadPrivateKey(cfg.FabricKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load key: %w", err)
	}

	id, err := identity.NewX509Identity(cfg.FabricMSPID, cert)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}

	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		return nil, fmt.Errorf("create sign: %w", err)
	}

	var dialOpts []grpc.DialOption
	if cfg.FabricTLSCertPath != "" {
		tlsCert, err := loadCertificate(cfg.FabricTLSCertPath)
		if err != nil {
			return nil, fmt.Errorf("load tls cert: %w", err)
		}
		certPool := x509.NewCertPool()
		certPool.AddCert(tlsCert)
		tlsCreds := credentials.NewClientTLSFromCert(certPool, "")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(tlsCreds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	grpcConn, err := grpc.Dial(cfg.FabricGatewayEndpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial: %w", err)
	}

	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(grpcConn),
		client.WithEvaluateTimeout(30*time.Second),
		client.WithEndorseTimeout(30*time.Second),
		client.WithSubmitTimeout(30*time.Second),
		client.WithCommitStatusTimeout(60*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to gateway: %w", err)
	}

	network := gw.GetNetwork(cfg.FabricChannel)
	chaincode := network.GetContract(cfg.FabricChaincode)

	return &Client{contract: chaincode}, nil
}

func (c *Client) devTxID() string {
	return "dev-" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func (c *Client) SubmitTransaction(fn string, args ...string) (string, error) {
	if c.devMode {
		return c.devTxID(), nil
	}
	resp, err := c.contract.SubmitTransaction(fn, args...)
	if err != nil {
		return "", fmt.Errorf("submit %s: %w", fn, err)
	}
	return string(resp), nil
}

func (c *Client) EvaluateTransaction(fn string, args ...string) ([]byte, error) {
	if c.devMode {
		return nil, fmt.Errorf("no fabric in dev mode")
	}
	resp, err := c.contract.EvaluateTransaction(fn, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluate %s: %w", fn, err)
	}
	return resp, nil
}

func (c *Client) CreateContract(id, clientID, freelancerID, title, briefHash string, amount, deposit float64, deadline string, revisions int) (string, error) {
	if c.devMode {
		return c.devTxID(), nil
	}
	return c.SubmitTransaction("CreateContract",
		id, clientID, freelancerID, title, briefHash,
		fmt.Sprintf("%.3f", amount), fmt.Sprintf("%.3f", deposit), deadline,
		fmt.Sprintf("%d", revisions),
	)
}

func (c *Client) FreelancerAccept(id string) (string, error) {
	return c.SubmitTransaction("FreelancerAccept", id)
}

func (c *Client) RecordDeposit(id, paymentRef string) (string, error) {
	return c.SubmitTransaction("RecordDeposit", id, paymentRef)
}

func (c *Client) ConfirmDeposit(id string) (string, error) {
	return c.SubmitTransaction("ConfirmDeposit", id)
}

func (c *Client) RecordDelivery(id, fileHash string) (string, error) {
	return c.SubmitTransaction("RecordDelivery", id, fileHash)
}

func (c *Client) ApproveDelivery(id string) (string, error) {
	return c.SubmitTransaction("ApproveDelivery", id)
}

func (c *Client) RequestRevision(id string) (string, error) {
	return c.SubmitTransaction("RequestRevision", id)
}

func (c *Client) RaiseDispute(id, reason string) (string, error) {
	return c.SubmitTransaction("RaiseDispute", id, reason)
}

func (c *Client) AdminResolve(id, decision string) (string, error) {
	return c.SubmitTransaction("AdminResolve", id, decision)
}

func (c *Client) AutoRelease(id string) (string, error) {
	return c.SubmitTransaction("AutoRelease", id)
}

func (c *Client) GetContract(id string) (*EscrowContract, error) {
	if c.devMode {
		return nil, fmt.Errorf("no fabric in dev mode — use database")
	}
	resp, err := c.EvaluateTransaction("GetContract", id)
	if err != nil {
		return nil, err
	}
	var ec EscrowContract
	if err := json.Unmarshal(resp, &ec); err != nil {
		return nil, fmt.Errorf("unmarshal contract: %w", err)
	}
	return &ec, nil
}
