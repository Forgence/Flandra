# Flandra

Flandra is a powerful tool designed to combine and analyze source code files across various programming languages. It allows you to combine multiple files of source code into a single file and provides the ability to extract functions and parameters.

## Features

- Combine multiple source code files into one.
- Extract functions and parameters including headers.
- Filter based on file type, size, and last modified date.
- Support for Golang, Rust, C#, Python, shell scripts, and more.
- Designed with extensibility in mind, allowing for easy addition of future functionality.

## Usage

```bash
go run main.go -directory=./myCodeDirectory -filetype=.go -size=500 -lastmodified=30 -subdirs=true -extract=all -out=output.txt
```

### Flags

- `-directory`: Specify the directory to start from. Defaults to the current directory.
- `-filetype`: Specify the type of files to consider. Defaults to all file types.
- `-size`: Filter files based on size (in KB). All files are considered if not specified.
- `-lastmodified`: Filter files based on the last modified date (in days). All files are considered if not specified.
- `-subdirs`: Whether to include subdirectories. Defaults to false.
- `-extract`: Ability to just extract all of the code or just the functions and any parameters they take (including headers and such). Options are "all" or "functions". Defaults to "all".
- `-out`: Output file to write the combined code. Defaults to "output.txt".

## Building the code

1. Clone the repository:

```bash
git clone https://github.com/Forgence/Flandra.git
```

2. Navigate into the directory:

```bash
cd Flandra
```

3. Build the application:

```bash
go build
```


## Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md) to get started.

## License

This project is licensed under the terms of the MIT license. See [LICENSE](LICENSE) for more details.