package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/suapapa/mqvision/internal/gemini"
	"github.com/suapapa/mqvision/internal/mqttdump"
)

var (
	mqttHostURI = "mqtt://mqtt:896351@192.168.219.105"
	mqttTopic   = "homin-home/gas-meter/cam"

	flagSingleShot = ""
	flagPort       = "8080"

	sensorServer *SensorServer
	geminiClient *gemini.Client

	chReadResult chan *gemini.GasMeterReadResult
)

func main() {
	var err error
	ctx := context.Background()

	log.Println("Creating Gemini client")
	geminiClient, err = gemini.NewClient(ctx, os.Getenv("GEMINI_API_KEY"))
	if err != nil {
		log.Fatalf("Error creating Gemini client: %v", err)
	}

	log.Println("Creating sensor server")
	sensorServer = &SensorServer{}

	chReadResult = make(chan *gemini.GasMeterReadResult, 10)
	defer close(chReadResult)
	go func() {
		for readResult := range chReadResult {
			// jsonBytes, err := json.MarshalIndent(readResult, "", "  ")
			// if err != nil {
			// 	log.Printf("Error marshalling read result: %v", err)
			// 	continue
			// }
			// os.Stdout.Write(jsonBytes)
			// os.Stdout.WriteString("\n")

			read, err := strconv.ParseFloat(readResult.Read, 64)
			if err != nil {
				log.Printf("Error parsing read value: %v", err)
				continue
			}
			sensorServer.SetValue(read, readResult)
			log.Printf("Updated sensor value: %s (%.3f)", readResult.Read, read)
		}
	}()

	flag.StringVar(&flagPort, "p", "8080", "Port to listen on")
	flag.StringVar(&flagSingleShot, "i", "", "Single run on image file")
	flag.Parse()

	go func() {
		if flagSingleShot != "" {
			imgFileNames, err := filepath.Glob(flagSingleShot)
			if err != nil {
				log.Fatalf("Error globbing image file: %v", err)
			}
			for _, imgFileName := range imgFileNames {
				log.Printf("Reading image file: %s", imgFileName)
				img, err := os.Open(imgFileName)
				if err != nil {
					log.Fatalf("Error opening image file: %v", err)
				}
				defer img.Close()
				readResult, err := geminiClient.ReadGasGuagePic(context.Background(), img)
				if err != nil {
					log.Fatalf("Error reading gauge image: %v", err)
				}

				chReadResult <- readResult
			}
		} else {
			log.Println("Running MQTT client")
			mqttClient, err := mqttdump.NewClient(mqttHostURI, mqttTopic)
			if err != nil {
				log.Fatalf("Error creating MQTT client: %v", err)
			}
			defer mqttClient.Stop()

			hdlr := mqttReadGuageSubHandler
			if err := mqttClient.Run(hdlr); err != nil {
				log.Fatalf("Error running MQTT client: %v", err)
			}

			log.Println("MQTT client running")
		}
	}()

	// gin.SetMode(gin.ReleaseMode)
	log.Printf("Starting Gin server on port %s", flagPort)
	router := gin.Default()
	router.GET("/sensor", sensorServer.GetValueHandler)
	if err := router.Run(":" + flagPort); err != nil {
		log.Fatalf("Error running Gin server: %v", err)
	}
}

func mqttReadGuageSubHandler() io.WriteCloser {
	pr, pw := io.Pipe()

	go func() {
		defer pr.Close()

		readResult, err := geminiClient.ReadGasGuagePic(context.Background(), pr)
		if err != nil {
			log.Printf("Error reading gauge image: %v", err)
			return
		}
		log.Printf("Read result: %+v", readResult)
		chReadResult <- readResult
	}()

	return pw
}

// func mqttFileDumpSubHandler() io.WriteCloser {
// 	timestamp := time.Now().Format("20060102_150405")
// 	filename := fmt.Sprintf("gauge_%s.jpg", timestamp)

// 	// Create output directory if it doesn't exist
// 	outputDir := "images"
// 	if err := os.MkdirAll(outputDir, 0755); err != nil {
// 		log.Printf("Error creating directory: %v", err)
// 		return nil
// 	}

// 	filepath := filepath.Join(outputDir, filename)

// 	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		log.Printf("Error opening file: %v", err)
// 		return nil
// 	}
// 	return f
// }
