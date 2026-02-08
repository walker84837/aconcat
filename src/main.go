package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

var (
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
	outputFile = flag.String("output", "", "Output audio file (required)")
	sampleRate = flag.Int("sample-rate", 48000, "Sample rate for re-encoding (default: 48000)")
	helpFlag   = flag.Bool("help", false, "Show usage information")
)

const (
	maxProgress                = 100
	progressReEncodeMultiplier = 2
	progressConcatMultiplier   = 1
	progressFinalMultiplier    = 2
)

var audioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true, ".aac": true,
	".ogg": true, ".m4a": true, ".wma": true, ".opus": true,
}

// validateInputFile checks if the input file exists, is readable, and has a valid audio extension
func validateInputFile(filePath string, logger *logrus.Logger) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %v", filePath, err)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("file does not exist: %s", absPath)
		}
		return fmt.Errorf("failed to access file %s: %v", absPath, err)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file: %s", absPath)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("file is not readable: %s", absPath)
	}
	file.Close()

	ext := strings.ToLower(filepath.Ext(absPath))
	if !audioExtensions[ext] {
		logger.Warnf("File %s does not have a common audio extension (%s)", absPath, ext)
	}

	return nil
}

// parseProgressLine extracts progress information from ffmpeg output and returns percentage complete
func parseProgressLine(line string, multiplier float64) int {
	progressRegex := regexp.MustCompile(`out_time_ms=(\d+)|time=(\d+):(\d+):(\d+\.\d+)`)
	matches := progressRegex.FindStringSubmatch(line)
	if len(matches) == 0 {
		return 0
	}

	if matches[1] != "" {
		if timeMs, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			// Progress in tenths of a second-per-multiplier
			progressTenths := int((float64(timeMs) / 1000000.0) * multiplier * 10)
			return min(progressTenths, maxProgress)
		}
	}

	if len(matches) < 5 {
		return 0
	}

	hours, err1 := strconv.Atoi(matches[2])
	minutes, err2 := strconv.Atoi(matches[3])
	seconds, err3 := strconv.ParseFloat(matches[4], 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}

	totalSeconds := float64(hours*3600+minutes*60) + seconds
	return min(int(totalSeconds*multiplier), maxProgress)
}

// runFFmpegWithProgress executes an ffmpeg command while displaying a progress bar and optional verbose output
func runFFmpegWithProgress(cmd *exec.Cmd, progressBar *progressbar.ProgressBar, verbose bool, multiplier float64) error {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	scanner := bufio.NewScanner(stderrPipe)
	for scanner.Scan() {
		line := scanner.Text()
		if verbose {
			fmt.Printf("\r%s", strings.Repeat(" ", 50))
			fmt.Printf("\rFFmpeg: %s", line)
		}

		if currentProgress := parseProgressLine(line, multiplier); currentProgress > 0 {
			progressBar.Set(currentProgress)
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg failed: %v", err)
	}

	progressBar.Set(maxProgress)
	fmt.Println()
	return nil
}

// logFileSize logs the size and location of a file when verbose mode is enabled
func logFileSize(filePath string, label string, verbose bool, logger *logrus.Logger) {
	if !verbose {
		return
	}

	if fileInfo, err := os.Stat(filePath); err == nil {
		logger.Infof("%s file size: %.2f MB", label, float64(fileInfo.Size())/1024/1024)
		logger.Infof("%s file location: %s", label, filePath)
	}
}

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

	// Validate all input files
	logger.Info("Validating input files...")
	for _, inputFile := range inputFiles {
		err := validateInputFile(inputFile, logger)
		if err != nil {
			logger.Fatalf("Input validation failed for %s: %v", inputFile, err)
		}
		logger.Infof("OK: %s", inputFile)
	}

	// Print list of files being processed in verbose mode
	if *verbose {
		logger.Info("Files to be processed:")
		for i, inputFile := range inputFiles {
			absPath, _ := filepath.Abs(inputFile)
			fileInfo, _ := os.Stat(absPath)
			logger.Infof("  %d. %s (%.2f MB)", i+1, absPath, float64(fileInfo.Size())/1024/1024)
		}
	}

	// Directory for storing re-encoded files
	tempDir := filepath.Join(os.TempDir(), "audio_concat")
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		logger.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if *verbose {
		logger.Infof("Temporary directory for re-encoded files: %s", tempDir)
	}

	var convertedFiles []string

	for i, inputFile := range inputFiles {
		absPath, err := filepath.Abs(inputFile)
		if err != nil {
			logger.Fatalf("Failed to get absolute path for %s: %v", inputFile, err)
		}

		convertedFile := filepath.Join(tempDir, filepath.Base(absPath)+"_converted.flac")
		logger.Infof("Re-encoding %s to %s", absPath, convertedFile)

		// Use the sample rate from the flag
		cmd := exec.Command("ffmpeg", "-i", absPath, "-ar", fmt.Sprintf("%d", *sampleRate), "-ac", "2", "-c:a", "flac", convertedFile)

		progressBar := progressbar.NewOptions(
			maxProgress,
			progressbar.OptionSetWidth(20),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetDescription(fmt.Sprintf("Re-encoding file %d/%d", i+1, len(inputFiles))),
		)

		cmd = exec.Command("ffmpeg", "-i", absPath, "-ar", "48000", "-ac", "2", "-c:a", "flac", "-progress", "-", convertedFile)

		if err := runFFmpegWithProgress(cmd, progressBar, *verbose, progressReEncodeMultiplier); err != nil {
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

	if *verbose {
		logger.Infof("Temporary concatenation list file: %s", listFile.Name())
	}

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

	logger.Info("Running ffmpeg to concatenate files.")
	flacFile := strings.TrimSuffix(*outputFile, filepath.Ext(*outputFile)) + ".flac"

	concatProgressBar := progressbar.NewOptions(maxProgress,
		progressbar.OptionSetWidth(20),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetDescription("Concatenating files"),
	)

	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFile.Name(), "-c", "copy", "-progress", "-", flacFile)

	if err := runFFmpegWithProgress(cmd, concatProgressBar, *verbose, progressConcatMultiplier); err != nil {
		logger.Fatalf("ffmpeg failed with error: %v", err)
	}

	logger.Infof("Concatenation of audio files is successful! Output file: %s", flacFile)
	logFileSize(flacFile, "Output", *verbose, logger)

	if strings.ToLower(filepath.Ext(*outputFile)) != ".flac" {
		finalOutput := *outputFile
		logger.Infof("Re-encoding %s to %s", flacFile, finalOutput)

		finalProgressBar := progressbar.NewOptions(maxProgress,
			progressbar.OptionSetWidth(20),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetDescription("Final encoding"))

		cmd = exec.Command("ffmpeg", "-i", flacFile, "-progress", "-", finalOutput)

		if err := runFFmpegWithProgress(cmd, finalProgressBar, *verbose, progressFinalMultiplier); err != nil {
			logger.Fatalf("ffmpeg failed to re-encode to %s: %v", finalOutput, err)
		}

		logger.Infof("Re-encoding to %s successful!", finalOutput)
		logFileSize(finalOutput, "Final output", *verbose, logger)
		os.Remove(flacFile)
	} else {
		logFileSize(flacFile, "Final output", *verbose, logger)
	}
}
