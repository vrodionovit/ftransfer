# File Transfer Application

This application is designed to handle file transfers using both SFTP and FTP protocols. It reads connection details from a YAML configuration file, splits the connections into multiple groups, and handles each connection group concurrently.

## Features
- Support for SSH key-based authentication for SFTP connections
- Establish connections to SFTP and FTP servers
- Recursively download files from remote directories
- Store details of downloaded files in a SQLite database
- Handle multiple connection groups concurrently
- Provide an HTTP server to serve information about downloaded files
- Validate connection configurations from a YAML file


## Configuration

The application reads connection details from a `connections.yaml` file. Below is an example of the configuration file:


```yaml
# connections: List of connection configurations
connections:
  - name: "sftp_conn_1"       # name: Unique name for the connection
    host: "127.0.0.1"         # host: Host address of the server
    port: 22                  # port: Port number to connect to
    protocol: "sftp"          # protocol: Protocol type (sftp or ftp or ftpoverssh)
    username: "seenisftp"     # username: Username for authentication
    password: "123qwe$$"      # password: Password for authentication
    delay: 5                  # delay: Delay in seconds between operations
    depth: 3                  # depth: Depth for recursive file download
    path: "."                 # path: Path to the directory on the server
    regex: "\\.(txt|jpeg)$"   # regex: Regular expression to match file types

  - name: "sftp_conn_3"
    host: "127.0.0.1"
    port: 22
    protocol: "sftp"
    username: "seenisftp"
    password: "123qwe$$"
    delay: 5
    depth: 3
    path: "upload"
    regex: "\\.(txt|jpeg)$"


  - name: "ftp_conn_1"
    host: "192.168.1.1"
    port: 21
    protocol: "ftp"
    username: "ftpuser"
    password: "ftppass"
```

## HTTP Endpoints

The application provides an HTTP server with the following endpoints:

- **GET /info**: Retrieves information about downloaded files from the database.
- **GET /connections**: Retrieves the list of connections from the YAML configuration file.
- **GET /health**: Health check endpoint to verify if the server is running.
- **POST /deleteOldEntries**: Deletes entries older than 7 days from the database.
- **POST /truncateDatabase**: Deletes all entries from the database.


### Example Usage

1. **Retrieve Downloaded Files Information:**
   ```sh
   curl http://localhost:8080/info
   ```

   This will return a JSON response with details about the downloaded files.

2. **Retrieve Connections:**
   ```sh
   curl http://localhost:8080/connections
   ```

   This will return a JSON response with the list of connections from the `connections.yaml` file.

3. **Health Check:**
   ```sh
   curl http://localhost:8080/health
   ```

   This will return a JSON response indicating the server status.


## Installation and Building

To install and build the project, follow these steps:

1. **Clone the repository:**
   ```sh
   git clone https://github.com/yourusername/ftransfer.git
   cd ftransfer
   ```

2. **Install dependencies:**
   Ensure you have Go installed on your machine. You can download it from [here](https://golang.org/dl/).

   ```sh
   go mod tidy
   ```

3. **Build the project:**
   ```sh
   go build -o ftransfer main.go
   ```

4. **Run the application:**
   ```sh
   ./ftransfer
   ```

   Ensure that you have the `connections.yaml` file in the same directory as the executable or provide the correct path to the configuration file.


5. **Run the application with flags:**
   You can run the application with various flags to customize its behavior. Here are the available flags:

   - `-port`: Specify the port for the HTTP server (default: 8080).
   - `-download`: Specify the directory for storing downloaded files (default: "download").
   - `-threads`: Specify the number of concurrent threads to use (default: 5).
   - `-truncate`: Specify whether to truncate the database before starting (default: false).
   - `-clean`: Specify whether to clean the download folder before starting (default: false).
   - `-keygen`: Specify whether to generate a new key before starting (default: false).
   - `-debug`: Specify whether to enable debug mode (default: false).

   Example usage:
   ```sh
   ./ftransfer -port=9090 -download=/path/to/download -threads=10 -truncate=true -clean=true -keygen=true
   ```



This command will output the version, commit hash, and build time of the application.



# Go Application Build Instructions

This document provides instructions on how to build, clean, run, and print version information for your Go application using the provided `Makefile`.

## Prerequisites

- Ensure you have Go installed on your system. You can download it from [golang.org](https://golang.org/dl/).
- Ensure you have `make` installed. It is typically available by default on Unix-like systems. On Windows, you can use tools like [MinGW](http://www.mingw.org/) or [Cygwin](https://www.cygwin.com/).

## Makefile Targets

### Build the Application

To build the application, run the following command:

```sh
make build
```


This command will compile the Go application and produce an executable named `myapp` (or the name specified in the `APP_NAME` variable).

### Clean the Build

To clean the build (i.e., remove the compiled executable), run the following command:
```sh
make clean
```


This command will remove the `myapp` executable from the current directory.

### Run the Application

To build and run the application, run the following command:


```sh
make run
```


This command will compile the Go application and then execute the resulting binary.

### Print Version Information

To print the version information of the application, run the following command:

```sh
make version
```


## Example

Here is an example of using the `Makefile`:

1. **Build the application**:
   ```sh
   make build
   ```

2. **Run the application**:
   ```sh
   make run
   ```

3. **Print version information**:
   ```sh
   make version
   ```

4. **Clean the build**:
   ```sh
   make clean
   ```
