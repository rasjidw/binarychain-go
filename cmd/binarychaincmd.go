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

	scr := binarychain.NewStreamingChainReader()
	for item := range scr.GetChainParts(*bc_data) {
		if item.ItemErr != nil {
			fmt.Printf("Got Error: %v\n", item.ItemErr)
		} else {
			fmt.Printf("Got: %v\n", item.Result)
		}
	}

	fmt.Print("---------------------\n")

	new_prefix := "AAA"
	bc.SetPrefix(&new_prefix)
	parts = []([]byte){[]byte{0x11, 0x22}}
	bc.Parts = &parts
	fmt.Println(bc)
	bc_data = bc.Serialise()
	fmt.Printf("Data as bytes (hex): % X\n", *bc_data)

	scr = binarychain.NewStreamingChainReader()
	scr.AddData(*bc_data)

	for {
		result := scr.GetNextItem()
		if result.ItemErr != nil {
			fmt.Printf("Got error: %v\n", err)
			break
		}

		if result.Result == nil {
			fmt.Print("Got nil")
			break
		}

		switch v := result.Result.(type) {
		case *binarychain.BinaryChainPrefix:
			fmt.Printf("Got Prefix: %v\n", v)
		case *binarychain.BinaryChainPart:
			fmt.Printf("Got Binary Part: %v\n", v)
		case *binarychain.EndOfChainMarker:
			fmt.Printf("Got EOC Marker: %v\n", v)
		case *binarychain.ParseError:
			// should never get this due to above
			fmt.Printf("Got parse error: %v", v)
		default:
			fmt.Printf("Default case: Got %v\n", v)
		}
	}
}
