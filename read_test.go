package imd

import (
	"fmt"
	"os"
	"testing"
)

var f, _ = os.Open("disk01.imd")

func TestX(t *testing.T) {
	f, err := Decode(f)
	fmt.Println(f.Comment)

	fmt.Println(err)
}
