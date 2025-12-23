package main

import (
	"binarychain"
	"fmt"
)

func main() {
	prefix := "A123"
	parts := []([]byte){[]byte{1, 2, 3, 4}, []byte{0x81, 0x82, 0x83, 0x90, 0x91}}
	bc, err := binarychain.NewBinaryChain(&prefix, &parts)
	if err != nil {
		fmt.Printf("Invalid binary chain: %v", err)
		return
	}

	fmt.Println(bc)
	bc_data := bc.Serialise()
	fmt.Printf("Data as bytes (hex): % X\n", *bc_data)
}
