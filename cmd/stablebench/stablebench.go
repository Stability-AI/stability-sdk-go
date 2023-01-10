package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/stability-ai/api-interfaces/gooseai/generation"
	"github.com/stability-ai/stability-sdk-go/stability_image"
	"github.com/stability-ai/stability-sdk-go/transport"
	"google.golang.org/grpc/metadata"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var grpcClient *generation.GenerationServiceClient
var endpointCtx context.Context

func buildResolutions(
	minPixels uint64,
	maxPixels uint64,
	dimStep uint64,
) stability_image.AspectRatioCollection {
	resolutions := make(stability_image.AspectRatioCollection, 0)
	resolutions = append(resolutions, stability_image.AspectRatio{
		Label:        "1:1",
		Width:        1,
		Height:       1,
		WidthPixels:  512,
		HeightPixels: 512,
	})
	ds := dimStep * dimStep

	seenPixels := make(map[uint64]bool)

	for currPixels := minPixels; currPixels <= maxPixels; currPixels += ds {
		aspects := stability_image.NewAspectRatios(currPixels, dimStep,
			64, 2048)

		smallestDim := uint64(0)
		largestDim := uint64(0)
		var smallestAspect *stability_image.AspectRatio
		var largestAspect *stability_image.AspectRatio
		for _, aspect := range aspects.Table {
			if smallestDim == 0 || aspect.WidthPixels < smallestDim {
				smallestDim = aspect.WidthPixels
				smallestAspect = &aspect
			}
			if largestDim == 0 || aspect.WidthPixels > largestDim {
				largestDim = aspect.WidthPixels
				largestAspect = &aspect
			}
			if smallestDim == 0 || aspect.HeightPixels < smallestDim {
				smallestDim = aspect.HeightPixels
				smallestAspect = &aspect
			}
			if largestDim == 0 || aspect.HeightPixels > largestDim {
				largestDim = aspect.HeightPixels
				largestAspect = &aspect
			}
		}

		if smallestAspect != nil {
			smallestPixels := smallestAspect.WidthPixels *
				smallestAspect.HeightPixels
			if !seenPixels[smallestPixels] {
				resolutions.InsertAspectFilteredByDimensions(smallestAspect)
				seenPixels[smallestPixels] = true
			}
		}

		if largestAspect != nil {
			largestPixels := largestAspect.WidthPixels *
				largestAspect.HeightPixels
			if !seenPixels[largestPixels] {
				resolutions.InsertAspectFilteredByDimensions(largestAspect)
				seenPixels[largestPixels] = true
			}
		}
	}
	// Sort resolutions by number of pixels
	resolutions.SortByResolution()
	return resolutions
}

var GuidancePresets = []generation.GuidancePreset{
	generation.GuidancePreset_GUIDANCE_PRESET_NONE,
	generation.GuidancePreset_GUIDANCE_PRESET_FAST_BLUE,
}

type Task struct {
	Id               uint64
	Prompt           string
	AspectRatio      stability_image.AspectRatio
	Steps            uint64
	NumSamples       uint64
	Preset           generation.GuidancePreset
	Engine           string
	Fanout           uint64
	Seed             uint32
	OutputDir        string
	CompletedSamples uint64
}

func (t *Task) String() string {
	return fmt.Sprintf("%d, %s, %s, %dx%d, %d, %d, %s, %s, %d",
		t.Id, t.Prompt, t.AspectRatio.Label, t.AspectRatio.WidthPixels,
		t.AspectRatio.HeightPixels, t.Steps, t.NumSamples,
		t.Preset, t.Engine, t.Seed)
}

func (t *Task) Request() generation.Request {
	prompts := []*generation.Prompt{
		{Prompt: &generation.Prompt_Text{
			Text: t.Prompt,
		}}}
	var guidance *generation.GuidanceParameters
	if t.Preset != generation.GuidancePreset_GUIDANCE_PRESET_NONE {
		guidance = &generation.GuidanceParameters{
			GuidancePreset: t.Preset,
		}
	}
	var numSamples uint64
	if t.NumSamples > t.Fanout {
		numSamples = t.Fanout
	} else {
		numSamples = t.NumSamples
	}
	return generation.Request{
		EngineId:      t.Engine,
		RequestedType: generation.ArtifactType_ARTIFACT_IMAGE,
		Prompt:        prompts,
		Params: &generation.Request_Image{
			Image: &generation.ImageParameters{
				Height:  &t.AspectRatio.HeightPixels,
				Width:   &t.AspectRatio.WidthPixels,
				Samples: &numSamples,
				Steps:   &t.Steps,
				Transform: &generation.TransformType{
					Type: &generation.TransformType_Diffusion{
						Diffusion: generation.DiffusionSampler_SAMPLER_K_DPM_2_ANCESTRAL,
					},
				},
				Parameters: []*generation.StepParameter{
					{ScaledStep: 0,
						Guidance: guidance},
				},
			},
		},
	}
}

func (t *Task) Run() error {
	request := t.Request()
	genResp, err := (*grpcClient).Generate(endpointCtx, &request)
	if err != nil {
		log.Printf("Error generating image: %s", err)
		if err.Error() == "rpc error: code = Unauthenticated desc = Bad"+
			" authorization string" {
			panic(err)
		}
		return err
	}
	for {
		answer, err := genResp.Recv()
		if err != nil && err != io.EOF {
			log.Printf("Error receiving image: %s", err)
			if err.Error() == "rpc error: code = Unauthenticated desc = Bad"+
				" authorization string" {
				panic(err)
			}
			return err
		}
		meta := answer.GetMeta()
		gpu_fields := strings.Split(meta.GetGpuId(), " ")
		gpu_id := gpu_fields[len(gpu_fields)-1]
		artifacts := answer.GetArtifacts()
		if artifacts != nil {
			compute := time.Duration(answer.Created-answer.Received) * time.Millisecond
			for idx, artifact := range artifacts {
				if artifact.Type == generation.
					ArtifactType_ARTIFACT_IMAGE {
					artifactData := artifact.GetBinary()
					filename := fmt.Sprintf(
						"generation-%dx%d-%s-%s-%d-%0.2f-%s-%s-%s-%d-%d.%s",
						t.AspectRatio.WidthPixels,
						t.AspectRatio.HeightPixels,
						strings.Replace(t.AspectRatio.Label,
							":", "_", -1),
						t.Preset.String(),
						t.Steps,
						compute.Seconds(),
						meta.GetNodeId(),
						gpu_id,
						answer.AnswerId,
						time.Now().Unix(),
						idx, "png")
					if err := ioutil.WriteFile(
						filepath.Join(t.OutputDir, filename),
						artifactData, 0644); err != nil {
						log.Printf("Error writing image: %s", err)
					} else {
						t.CompletedSamples += 1
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	return nil
}

type TaskList []Task
type TaskQueue chan Task

func (t TaskList) Run(concurrency uint64) {
	log.Printf("Running %d tasks with %d concurrent workers\n",
		len(t), concurrency)
	queue := make(TaskQueue, len(t))
	for _, task := range t {
		queue <- task
	}
	close(queue)
	var wg sync.WaitGroup
	for i := uint64(1); i < concurrency+1; i++ {
		wg.Add(1)
		go queue.RunWorker(i, &wg)
		time.Sleep(30 * time.Second)
	}
	wg.Wait()
}

func (queue TaskQueue) RunWorker(workerId uint64, wg *sync.WaitGroup) {
	defer wg.Done()
	for task := range queue {
		log.Printf("Worker %d: running task %s\n", workerId,
			task.String())
		err := errors.New("placeholder")
		errCtr := 0
		start := time.Now()
		for err != nil || task.CompletedSamples < task.NumSamples {
			err = task.Run()
			if err != nil {
				errCtr++
				log.Printf("Worker %d: task %d failed %d times, %d left",
					workerId, task.Id, errCtr, task.NumSamples-task.
						CompletedSamples)
				errorReport := fmt.Sprintf("Worker: %d Task: %s Error: %s\n",
					workerId, task, err)
				log.Printf(errorReport)
				errorLog, _ := os.OpenFile(task.OutputDir+"/error.log",
					os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				_, _ = errorLog.WriteString(errorReport)
				_ = errorLog.Close()
				time.Sleep(20 * time.Second)
				if errCtr > 10 {
					err = nil
				}
			} else if task.CompletedSamples >= task.NumSamples {
				log.Printf("Worker %d: task %d succeeded with %d"+
					"artifacts in %s\n",
					workerId, task.Id, task.CompletedSamples,
					time.Since(start).Round(time.Millisecond))
				time.Sleep(1 * time.Second)
				break
			}
		}
	}
}

func buildTaskList(
	resolutions stability_image.AspectRatioCollection,
	prompt string,
	startSteps uint64,
	maxSteps uint64,
	stepIncrement uint64,
	numSamples uint64,
	fanoutSamples uint64,
	engine string,
	outputDir string,
) TaskList {
	taskList := make(TaskList, 0)
	resolutions.SortByResolution()
	seed := uint32(0)
	for seed != 0 {
		seed = rand.Uint32()
	}
	ctr := uint64(1)
	stepsArr := make([]uint64, 0)
	stepsArr = append(stepsArr, 10)
	for steps := startSteps; steps < maxSteps+1; steps += stepIncrement {
		stepsArr = append(stepsArr, steps)
	}

	for _, aspect := range resolutions {
		for _, steps := range stepsArr {
			for _, preset := range GuidancePresets {
				taskList = append(taskList, Task{
					Id:          ctr,
					Prompt:      prompt,
					AspectRatio: aspect,
					Steps:       steps,
					NumSamples:  numSamples,
					Preset:      preset,
					Engine:      engine,
					OutputDir:   outputDir,
					Fanout:      fanoutSamples,
					Seed:        rand.Uint32()})
				ctr++
			}
		}
	}
	return taskList
}

func main() {
	prompt := flag.String("prompt", "A starlit sky.",
		"text prompt to use")
	maxPixels := flag.Uint64("pixels", 1088*1024,
		"maximum number of pixels to use")
	minPixels := flag.Uint64("min-pixels", 384*384,
		"minimum number of pixels to use")
	dimStep := flag.Uint64("dim-step", 128,
		"step size for image dimensions")
	startSteps := flag.Uint64("start-steps", 50,
		"number of steps to start with")
	maxSteps := flag.Uint64("max-steps", 150,
		"maximum number of steps to use")
	stepIncrement := flag.Uint64("step-increment", 50,
		"number of steps to increment by")
	numSamples := flag.Uint64("num-samples", 10,
		"number of samples to take")
	outputDirectory := flag.String("output", "output",
		"directory to output images to")
	engingeId := flag.String("engine", "stable-diffusion-v1-5",
		"engine id to use")
	concurrency := flag.Uint64("concurrency", 1,
		"number of concurrent workers to use")
	batchSize := flag.Uint64("batch-size", 10,
		"number of samples to request per request")
	endpoint := flag.String("endpoint",
		"https://grpc-staging.stability.ai:443/",
		"endpoint to use")
	flag.Parse()
	auth := os.Getenv("STABILITY_KEY")
	if auth == "" {
		log.Println("WARNING: STABILITY_KEY not set")
	}
	log.Printf("prompt: %s\n", *prompt)
	log.Printf("maxPixels: %d\n", *maxPixels)
	log.Printf("minPixels: %d\n", *minPixels)
	log.Printf("dimStep: %d\n", *dimStep)
	resolutions := buildResolutions(*minPixels, *maxPixels, *dimStep)

	for _, aspect := range resolutions {
		log.Printf("%s: %d x %d, %d pixels\n", aspect.Label,
			aspect.WidthPixels, aspect.HeightPixels,
			aspect.WidthPixels*aspect.HeightPixels)
	}
	// Set up our gRPC context with our Stability.AI API key
	md := metadata.New(map[string]string{"authorization" +
		"": "Bearer " + auth})
	endpointCtx = metadata.NewOutgoingContext(context.Background(), md)
	grpcClient, _ = transport.ConnectGrpc(*endpoint, auth)

	log.Printf("Found %d resolutions\n", len(resolutions))
	taskList := buildTaskList(resolutions, *prompt, *startSteps, *maxSteps,
		*stepIncrement, *numSamples, *batchSize, *engingeId,
		*outputDirectory)
	log.Printf("Found %d tasks\n", len(taskList))
	log.Printf("outputDirectory: %s\n", *outputDirectory)
	if mkdirErr := os.MkdirAll(*outputDirectory, 0755); mkdirErr != nil {
		log.Fatalf("Error creating output directory: %s", mkdirErr)
	}
	taskList.Run(*concurrency)
}
