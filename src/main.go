package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

var (
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	outputFile = flag.String("output", "", "Output audio file (required)")
	sampleRate = flag.Int("sample-rate", 48000, "Sample rate for re-encoding (default: 48000)")
	helpFlag   = flag.Bool("help", false, "Show usage information")
)

func usage() {
	fmt.Println(`Usage: aconcat [options] <input-file-1> <input-file-2> ...

aconcat is a command-line tool for concatenating multiple audio files into one output file.
It re-encodes the input files to a common format (FLAC by default) before concatenation.

Options:
  -verbose        Enable verbose logging
  -output         Specify the output audio file (required)
  -sample-rate    Set the sample rate for re-encoding (default: 48000)
  -help           Show this help message

Examples:
  aconcat -output final_audio.wav file1.mp3 file2.wav
  aconcat -sample-rate 44100 -output final.flac file1.aac file2.ogg`)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *helpFlag {
		flag.Usage()
		return
	}

	inputFiles := flag.Args()
	if len(inputFiles) < 2 || *outputFile == "" {
		logrus.Error("Error: You must provide at least two input files and specify an output file.")
		flag.Usage()
		os.Exit(1)
	}

	logger := logrus.New()
	if *verbose {
		logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
		logger.SetLevel(logrus.InfoLevel)
	} else {
		logger.SetLevel(logrus.WarnLevel)
	}

	// Directory for storing re-encoded files
	tempDir := filepath.Join(os.TempDir(), "audio_concat")
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		logger.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var convertedFiles []string

	// Re-encode input files to a common format and codec
	for _, inputFile := range inputFiles {
		absPath, err := filepath.Abs(inputFile)
		if err != nil {
			logger.Fatalf("Failed to get absolute path for %s: %v", inputFile, err)
		}

		convertedFile := filepath.Join(tempDir, filepath.Base(absPath)+"_converted.flac")
		logger.Infof("Re-encoding %s to %s", absPath, convertedFile)

		// Use the sample rate from the flag
		cmd := exec.Command("ffmpeg", "-i", absPath, "-ar", fmt.Sprintf("%d", *sampleRate), "-ac", "2", "-c:a", "flac", convertedFile)

		if !*verbose {
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
		}

		err = cmd.Run()
		if err != nil {
			logger.Fatalf("ffmpeg failed to re-encode %s: %v", absPath, err)
		}

		convertedFiles = append(convertedFiles, convertedFile)
	}

	// Create a temporary file for the concatenation list
	logger.Info("Creating temporary file for concatenation list.")
	listFile, err := os.CreateTemp("", "concat-list-*.txt")
	if err != nil {
		logger.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(listFile.Name())
	logger.Infof("Temporary file created at: %s", listFile.Name())

	// Write re-encoded files to the temporary file
	for _, file := range convertedFiles {
		_, err := fmt.Fprintf(listFile, "file '%s'\n", file)
		if err != nil {
			logger.Fatalf("Failed to write to temporary file list: %v", err)
		}
	}

	// Print out the content of the temporary file for verification
	listFile.Seek(0, io.SeekStart)
	content, err := io.ReadAll(listFile)
	if err != nil {
		logger.Fatalf("Failed to read temporary file: %v", err)
	}
	logger.Infof("Temporary file content:\n%s", content)

	// Run ffmpeg to concatenate re-encoded files
	logger.Info("Running ffmpeg to concatenate files.")
	flacFile := strings.TrimSuffix(*outputFile, filepath.Ext(*outputFile)) + ".flac"
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFile.Name(), "-c", "copy", flacFile)

	if !*verbose {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	// Create a progress bar
	progressBar := progressbar.NewOptions(100,
		progressbar.OptionSetWidth(20),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription("Processing"))

	// Simulate progress update
	go func() {
		for i := 0; i < 100; i++ {
			if !*verbose {
				progressBar.Add(1)
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()

	err = cmd.Run()
	if err != nil {
		logger.Fatalf("ffmpeg failed with error: %v", err)
	}

	logger.Infof("Concatenation of audio files is successful! Output file: %s", flacFile)

	// Check the extension of the output file
	if strings.ToLower(filepath.Ext(*outputFile)) != ".flac" {
		// Re-encode to the desired output format
		finalOutput := *outputFile
		logger.Infof("Re-encoding %s to %s", flacFile, finalOutput)
		cmd = exec.Command("ffmpeg", "-i", flacFile, finalOutput)

		if !*verbose {
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
		}

		err = cmd.Run()
		if err != nil {
			logger.Fatalf("ffmpeg failed to re-encode to %s: %v", finalOutput, err)
		}
		logger.Infof("Re-encoding to %s successful!", finalOutput)
		os.Remove(flacFile)
	}
}
