# aconcat: audio concatenation tool

A CLI tool for concatenating multiple audio files into a single output file.

This tool provides a way to concatenate multiple audio files into one. It
re-encodes all input audio files to a common format before concatenation.
This ensures compatibility and consistency across all files. The tool uses
`ffmpeg` for both re-encoding and concatenation processes. It supports verbose
logging to assist in debugging and verification.

## Table of Contents

  - [Install](#install)
  - [Usage](#usage)
  - [Contributing](#contributing)
  - [License](#license)

## Install

### Dependencies

Ensure you have `ffmpeg` installed on your system, as this tool relies on it for
audio processing. You can install `ffmpeg` from [FFmpeg's official
website](https://ffmpeg.org/download.html) or through a package manager.

### Installation

To use this tool, simply clone the repository and build it using Go:

``` console
$ git clone https://github.com/walker84837/aconcat.git
$ cd aconcat
$ make
```

## Usage

### CLI

To run the tool, use the following command:

``` console
$ ac [flags] <input-files>...
```

**Flags:**

  - `-verbose`: Enable verbose logging.
  - `-output <file>`: Specify the output audio file.

**Examples:**

1.  Basic usage without verbose logging:
    
    ``` console
    $ ac -output combined.flac file1.mp3 file2.wav
    ```

2.  Usage with verbose logging:
    
    ``` console
    $ ac -verbose -output combined.flac file1.mp3 file2.wav
    ```

**Note:** You must provide at least two input files for concatenation.

## Contributing

Contributions are welcome! If you have suggestions or improvements, please
submit a pull request or open an issue on the GitHub repository.

For any questions or discussions, you can reach out via the repository's issue
tracker.

## License

This project is licensed under the [BSD 3-Clause License](LICENSE.md) file for
details.
