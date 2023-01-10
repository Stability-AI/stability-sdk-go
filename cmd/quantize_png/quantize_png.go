package main

import (
	"fmt"
	"github.com/stability-ai/stability-sdk-go/metadata"
	"github.com/stability-ai/stability-sdk-go/stability_image"
	"github.com/yargevad/filepathx"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Given a directory path, perform glob expansion and return a list of paths.
func getPngPaths(path string) ([]string, error) {
	derived := path + "/**/*.png"
	paths, err := filepathx.Glob(derived)
	if err != nil {
		return nil, err
	}
	return paths, nil
}

// Giving a list of paths, recursively turn a list of paths that are PNGs.
func getPaths(args []string) []string {
	if len(args) < 1 {
		fmt.Println("Usage: repng <outputdir> <file> [file...]")
		os.Exit(1)
	}
	paths := []string{}
	for _, arg := range args {
		newPaths, err := getPngPaths(arg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		paths = append(paths, newPaths...)
	}
	return paths
}

type QuantizationTask struct {
	Path     string
	Png      *[]byte
	OrigSize int
	Error    error
}

func (qt *QuantizationTask) Run() {
	rq, decodeErr := metadata.DecodeRequest(qt.Png)
	if decodeErr != nil {
		qt.Error = decodeErr
	} else {
		reencoded, encodeErr := stability_image.QuantizePng(qt.Png,
			8)
		if encodeErr != nil {
			qt.Error = encodeErr
		} else {
			embedded, embedErr := metadata.EmbedRequest(rq, reencoded)
			if embedErr != nil {
				qt.Error = embedErr
			} else {
				qt.Png = embedded
			}
		}
	}
}

func QuantizationWorker(
	tasks <-chan *QuantizationTask,
	results chan<- *QuantizationTask,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for task := range tasks {
		task.Run()
		results <- task
	}
}

func QuantizationResultWriter(
	output string,
	results <-chan *QuantizationTask,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for result := range results {
		if result.Error != nil {
			fmt.Println(result.Error)
		} else {
			filename := filepath.Base(result.Path)
			writeErr := ioutil.WriteFile(output+"/"+filename, *result.Png,
				0644)
			if writeErr != nil {
				fmt.Println(writeErr)
				os.Exit(1)
			}
			compressionResult := float64(len(*result.Png)) / float64(result.
				OrigSize) * 100
			fmt.Println(fmt.Sprintf("%s: %0.2f -- %d -> %d", result.Path,
				compressionResult, result.OrigSize, len(*result.Png)))
		}
	}
}

func StartQuantizationWorkers(
	tasks <-chan *QuantizationTask,
	results chan *QuantizationTask,
	output string,
	numWorkers int,
) *sync.WaitGroup {
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go QuantizationWorker(tasks, results, &wg)
	}
	return &wg
}

func StartQuantizationResultWriter(
	results <-chan *QuantizationTask,
	output string,
) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go QuantizationResultWriter(output, results, &wg)
	return &wg
}

func main() {
	output := os.Args[1]
	mkdirErr := os.MkdirAll(output, 0755)
	if mkdirErr != nil {
		fmt.Println(mkdirErr)
		os.Exit(1)
	}

	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	numWorkers := maxProcs
	if numWorkers > numCPU {
		numWorkers = numCPU
	}

	tasks := make(chan *QuantizationTask, numWorkers)
	results := make(chan *QuantizationTask, numWorkers)
	wg := StartQuantizationWorkers(tasks, results, output, numWorkers)
	writerWg := StartQuantizationResultWriter(results, output)

	paths := getPaths(os.Args[2:])
	for _, path := range paths {
		// Check if the file exists in the output directory.
		// If it does, skip it.
		_, err := os.Stat(output + "/" + filepath.Base(path))
		if err == nil {
			fmt.Println("Skipping " + path)
			continue
		}
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		tasks <- &QuantizationTask{
			Path:     path,
			Png:      &contents,
			OrigSize: len(contents),
		}
	}
	close(tasks)
	wg.Wait()
	close(results)
	writerWg.Wait()
}
