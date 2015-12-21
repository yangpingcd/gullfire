package main

import (
	"fmt"
	"testing"
)

func TestMem(*testing.T) {
	fmt.Println("total:", mem_GetTotal())
	fmt.Println("available:", mem_GetAvailable())
}
