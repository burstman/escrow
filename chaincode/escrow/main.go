package main

import (
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

func main() {
	cc := new(SmartContract)
	chaincode, err := contractapi.NewChaincode(cc)
	if err != nil {
		panic(err)
	}
	if err := chaincode.Start(); err != nil {
		panic(err)
	}
}
