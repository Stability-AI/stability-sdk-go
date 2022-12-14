package interrogate

import (
	"fmt"
	"github.com/stability-ai/stability-sdk-go/metadata"
	"google.golang.org/protobuf/encoding/prototext"
	"io/ioutil"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: interrogate <file>")
		os.Exit(1)
	}
	filepath := os.Args[1]
	contents, err := ioutil.ReadFile(filepath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	rq, decodeErr := metadata.DecodeRequest(&contents)
	if decodeErr != nil {
		fmt.Println(fmt.Sprintf("WARNING: %v", decodeErr))
	}
	metadata.RemoveBinaryData(rq)
	t := prototext.Format(rq)
	fmt.Println(t)
}
