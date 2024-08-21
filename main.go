package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/sirupsen/logrus"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

// These variables will be set at build time
var (
	version   string
	buildTime string
)

// Define a struct to match the structure of your connections.yaml
type Connection struct {
	Name       string `yaml:"name"`
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Protocol   string `yaml:"protocol"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Delay      int    `yaml:"delay"`
	Path       string `yaml:"path"`
	Depth      int    `yaml:"depth"`
	Regex      string `yaml:"regex"`
	SSHKeyPath string `yaml:"sshkeypath"`
	Status     bool   `yaml:"status,omitempty" default:"false"`
}

type Config struct {
	Connections []Connection `yaml:"connections"`
}

type ManagerSFTP struct {
	sftpClient *sftp.Client
	sshConn    *ssh.Client
}

type ManagerFTP struct {
	ftpConn *ftp.ServerConn
}

type ManagerFTPoverSSH struct {
	ftpConn *ftp.ServerConn
	sshConn *ssh.Client
}

type SplittedConnections struct {
	Connection map[string][]Connection
}

type DB struct {
	conn *sql.DB
	mu   sync.Mutex
}

type Manager interface {
	connect(conn Connection) error
	downloadFile(remotePath, localPath string) error
	deleteFile(remotePath string) error
	readDir(remotePath string) ([]*ftp.Entry, error)
}

var db DB

var Connections = SplittedConnections{}
var download_folder = ""

var logger *logrus.Logger = setupLogger()

func setupLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:            true,                  // Enable colors in the output
		FullTimestamp:          true,                  // Include full timestamp in the log output
		TimestampFormat:        "2006-01-02 15:04:05", // Set the format for the timestamp
		DisableLevelTruncation: true,                  // Disable truncation of the log level text
		QuoteEmptyFields:       true,                  // Quote empty fields in the log output
	})
	return logger
}

func readConfig(path string) (Config, error) {
	// Read the YAML file

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("error reading file: %v", err)
	}

	// Parse the YAML file
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, fmt.Errorf("error parsing YAML: %v", err)
	}

	// Validate the fields
	for _, conn := range config.Connections {
		if conn.Name == "" {
			return Config{}, fmt.Errorf("connection name is missing")
		}
		if conn.Host == "" {
			return Config{}, fmt.Errorf("connection host is missing for %s", conn.Name)
		}
		if conn.Port <= 0 || conn.Port > 65535 {
			return Config{}, fmt.Errorf("invalid port number for %s: %d", conn.Name, conn.Port)
		}
		if conn.Protocol != "sftp" && conn.Protocol != "ftp" && conn.Protocol != "ftpoverssh" {
			return Config{}, fmt.Errorf("unsupported protocol for %s: %s", conn.Name, conn.Protocol)
		}
		if conn.Username == "" {
			return Config{}, fmt.Errorf("username is missing for %s", conn.Name)
		}
		if conn.Password == "" {
			return Config{}, fmt.Errorf("password is missing for %s", conn.Name)
		}
		if conn.Delay < 0 {
			return Config{}, fmt.Errorf("invalid delay for %s: %d", conn.Name, conn.Delay)
		}
		if conn.Path == "" {
			return Config{}, fmt.Errorf("path is missing for %s", conn.Name)
		}
		if conn.Depth < 0 {
			return Config{}, fmt.Errorf("invalid depth for %s: %d", conn.Name, conn.Depth)
		}

	}

	return config, nil
}

func splitConnections(config Config, numSplits int) SplittedConnections {
	splitted := SplittedConnections{Connection: make(map[string][]Connection)}
	totalConnections := len(config.Connections)
	connectionsPerSplit := totalConnections / numSplits

	for i := 0; i < numSplits; i++ {
		start := i * connectionsPerSplit
		end := start + connectionsPerSplit
		if i == numSplits-1 {
			end = totalConnections // Ensure the last split gets any remaining connections
		}
		splitted.Connection[fmt.Sprintf("group_%d", i+1)] = config.Connections[start:end]
	}

	return splitted
}

// func sendEmail(to, subject, body string) error {

// 	// Set up the email server configuration.
// 	var from string = "coldplugy@gmail.com"
// 	var password string = "Wyg6j{}h"

// 	var smtpServer string = "smtp.gmail.com"
// 	var smtpPort int = 587

// 	// Set up authentication information.
// 	auth := smtp.PlainAuth("", from, password, smtpServer)

// 	// Compose the email message.
// 	msg := []byte("To: " + to + "\r\n" +
// 		"From: " + from + "\r\n" +
// 		"Subject: " + subject + "\r\n" +
// 		"MIME-version: 1.0;\r\n" +
// 		"Content-Type: text/html; charset=\"UTF-8\";\r\n" +
// 		"\r\n" +
// 		body + "\r\n")

// 	// // Send the email without authentication.
// 	// err := smtp.SendMail(fmt.Sprintf("%s:%d", smtpServer, smtpPort), nil, from, []string{to}, msg)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to send email: %v", err)
// 	// }

// 	// Send the email without authentication.
// 	err := smtp.SendMail(fmt.Sprintf("%s:%d", smtpServer, smtpPort), auth, from, []string{to}, msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send email: %v", err)
// 	}

// 	logger.Infof("Email sent successfully to %s", to)
// 	return nil
// }

func bytesToHumanReadable(bytes int64) string {
	const (
		_        = iota
		kilobyte = 1 << (10 * iota)
		megabyte
		gigabyte
		terabyte
		petabyte
	)

	switch {
	case bytes >= petabyte:
		return fmt.Sprintf("%.2f PB", float64(bytes)/petabyte)
	case bytes >= terabyte:
		return fmt.Sprintf("%.2f TB", float64(bytes)/terabyte)
	case bytes >= gigabyte:
		return fmt.Sprintf("%.2f GB", float64(bytes)/gigabyte)
	case bytes >= megabyte:
		return fmt.Sprintf("%.2f MB", float64(bytes)/megabyte)
	case bytes >= kilobyte:
		return fmt.Sprintf("%.2f KB", float64(bytes)/kilobyte)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func recursivelyDownloadSFTP(remotePath, localPath string, depth int, fm ManagerSFTP, conn Connection) error {
	if depth == 0 {
		return nil
	}

	// Read the directory contents from the remote SFTP server
	files, err := fm.sftpClient.ReadDir(remotePath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}
	for _, file := range files {
		logger.Debugf("Found file: %s, Size: %s, IsDir: %t\n", file.Name(), bytesToHumanReadable(file.Size()), file.IsDir())
	}
	for _, file := range files {
		remoteFilePath := path.Join(remotePath, file.Name())
		localFilePath := path.Join(localPath, file.Name())

		if file.IsDir() {
			// If the file is a directory, create the corresponding local directory
			if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
				err := os.MkdirAll(localFilePath, os.ModePerm)
				if err != nil {
					logger.Debugf("Error creating directory: %v\n", err)
					continue
				}
			}
			// Recursively download the contents of the directory
			err := recursivelyDownloadSFTP(remoteFilePath, localFilePath, depth-1, fm, conn)
			if err != nil {
				logger.Debugf("Error downloading directory: %v\n", err)
				continue
			}
		} else {

			// Check if the file matches the regex mask if regex is provided
			matched := true
			if conn.Regex != "" {
				matched, err = regexp.MatchString(conn.Regex, file.Name())
				if err != nil {
					logger.Debugf("Error matching regex: %v\n", err)
					continue
				}
			}

			if !matched {
				logger.Debugf("File does not match the regex mask: %s\n", file.Name())
				continue
			}
			// Check if the file has already been downloaded
			db.mu.Lock()
			existingFiles, err := searchDownloadedFileEntries(db.conn, file.Name(), file.Size(), conn.Name)
			db.mu.Unlock()

			if err != nil {
				logger.Debugf("Error searching for existing file entries: %v\n", err)
				continue
			}

			if len(existingFiles) > 0 {
				logger.Warnf("File already downloaded: %s\n", file.Name())
				continue
			}
			// If the file is not a directory, download the file
			srcFile, err := fm.sftpClient.Open(remoteFilePath)
			if err != nil {
				logger.Debugf("Error opening source file: %v\n", err)
				continue
			}
			defer srcFile.Close()

			dstFile, err := os.Create(localFilePath)
			if err != nil {
				logger.Debugf("Error creating destination file: %v\n", err)
				continue
			}
			defer dstFile.Close()

			// Calculate the download time
			startTime := time.Now()

			// Copy the file contents from the remote file to the local file
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				logger.Debugf("Error copying file: %v\n", err)
				continue
			}

			downloadTime := time.Since(startTime)
			logger.Infof("Downloaded file: %s in %v\n", file.Name(), downloadTime)

			// Check file sizes to ensure the download was successful
			srcFileInfo, err := srcFile.Stat()
			if err != nil {
				logger.Debugf("Error getting source file info: %v\n", err)
				continue
			}

			dstFileInfo, err := dstFile.Stat()
			if err != nil {
				logger.Debugf("Error getting destination file info: %v\n", err)
				continue
			}

			if srcFileInfo.Size() != dstFileInfo.Size() {
				logger.Debugf("File size mismatch for %s: source size %d, destination size %d\n", file.Name(), srcFileInfo.Size(), dstFileInfo.Size())
				// If the file sizes do not match, delete the local file
				err = os.Remove(localFilePath)
				if err != nil {
					logger.Debugf("Error deleting invalid local file: %v\n", err)
				} else {
					logger.Debugf("Deleted invalid local file: %s\n", localFilePath)
				}
			} else {

				logger.Debugf("File size match for %s: %d bytes\n", file.Name(), srcFileInfo.Size())

				// If the file sizes match, delete the file from the server
				err = fm.sftpClient.Remove(remoteFilePath)
				if err != nil {
					logger.Errorf("Error deleting file from server: %v\n", err)
				} else {
					logger.Debugf("Deleted file from server: %s\n", remoteFilePath)
				}

				// Create a sample downloaded file entry
				file := DownloadedFile{
					FileName:     file.Name(),
					ServerName:   conn.Name,
					FileSize:     dstFileInfo.Size(),
					DownloadTime: time.Now().Format("2006-01-02 15:04:05"),
				}

				// Save the downloaded file entry to the database
				db.mu.Lock()
				err = saveDownloadedFileEntry(db.conn, file)
				db.mu.Unlock()
				if err != nil {
					logger.Fatalf("Failed to save file entry: %v", err)
				}
			}
		}
	}
	return nil
}

func recursivelyDownloadFTP(remotePath, localPath string, depth int, fm Manager, conn Connection) error {
	if depth == 0 {
		return nil
	}

	files, err := fm.readDir(remotePath)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}
	for _, file := range files {
		logger.Debugf("Found file: %s, Size: %s, IsDir: %t\n", file.Name, bytesToHumanReadable(int64(file.Size)), file.Type == ftp.EntryTypeFolder)
	}

	for _, file := range files {
		remoteFilePath := path.Join(remotePath, file.Name)
		localFilePath := path.Join(localPath, file.Name)

		if file.Type == ftp.EntryTypeFolder {
			// If the file is a directory, create the corresponding local directory
			if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
				err := os.MkdirAll(localFilePath, os.ModePerm)
				if err != nil {
					logger.Debugf("Error creating directory: %v\n", err)
					continue
				}
			}
			// Recursively download the contents of the directory
			err := recursivelyDownloadFTP(remoteFilePath, localFilePath, depth-1, fm, conn)
			if err != nil {
				logger.Debugf("Error downloading directory: %v\n", err)
				continue
			}
		} else {

			// Check if the file matches the regex mask if regex is provided
			matched := true
			if conn.Regex != "" {
				matched, err = regexp.MatchString(conn.Regex, file.Name)
				if err != nil {
					logger.Debugf("Error matching regex: %v\n", err)
					continue
				}
			}

			if !matched {
				logger.Debugf("File does not match the regex mask: %s\n", file.Name)
				continue
			}
			// Check if the file has already been downloaded
			db.mu.Lock()
			existingFiles, err := searchDownloadedFileEntries(db.conn, file.Name, int64(file.Size), conn.Name)
			db.mu.Unlock()

			if err != nil {
				logger.Debugf("Error searching for existing file entries: %v\n", err)
				continue
			}

			if len(existingFiles) > 0 {
				logger.Warnf("File already downloaded: %s\n", file.Name)
				continue
			}

			// Simulate downloading the file from the FTP server
			err = fm.downloadFile(remoteFilePath, localFilePath)
			if err != nil {
				logger.Debugf("Error downloading file: %v\n", err)
				continue
			}

			// Verify the file size to ensure the download was successful
			localFileInfo, err := os.Stat(localFilePath)
			if err != nil {
				logger.Debugf("Error stating local file: %v\n", err)
				continue
			}

			if localFileInfo.Size() != int64(file.Size) {
				logger.Debugf("File size mismatch for %s: expected %d, got %d\n", file.Name, file.Size, localFileInfo.Size())
				// If the file sizes do not match, delete the local file
				err = os.Remove(localFilePath)
				if err != nil {
					logger.Debugf("Error deleting invalid local file: %v\n", err)
				} else {
					logger.Debugf("Deleted invalid local file: %s\n", localFilePath)
				}
			} else {
				// If the file sizes match, delete the file from the server
				err = fm.deleteFile(remoteFilePath)
				if err != nil {
					logger.Debugf("Error deleting file from server: %v\n", err)
				}

				// Create a sample downloaded file entry
				downloadedFile := DownloadedFile{
					FileName:     file.Name,
					ServerName:   conn.Name,
					FileSize:     int64(file.Size),
					DownloadTime: time.Now().Format("2006-01-02 15:04:05"),
				}

				// Save the downloaded file entry to the database
				db.mu.Lock()
				err = saveDownloadedFileEntry(db.conn, downloadedFile)
				db.mu.Unlock()
				if err != nil {
					logger.Fatalf("Failed to save file entry: %v", err)
				}

			}

		}
	}
	return nil
}

func handleFTPoverSSH(conn Connection) {
	var fm ManagerFTPoverSSH = ManagerFTPoverSSH{}

	// Attempt to connect to the FTPoverSSH server
	err := fm.connect(conn)
	if err != nil {
		logger.Debugf("Error connecting to FTPoverSSH: %v\n", err)
		return
	}

	// Define the source folder for downloads
	var srcFolder string = conn.Path

	// Define the local directory for downloads
	localDir := path.Join(download_folder, conn.Name)
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		// Create the local directory if it does not exist
		err := os.MkdirAll(localDir, os.ModePerm)
		if err != nil {
			logger.Debugf("Error creating directory: %v\n", err)
			return
		}
	}

	// Start downloading files from the source folder to the local directory
	// implement this
	err = recursivelyDownloadFTP(srcFolder, localDir, conn.Depth, &fm, conn) // Adjust the depth as needed
	if err != nil {
		logger.Debugf("Error downloading files: %v\n", err)
	}

	// Close connections
	fm.ftpConn.Quit()
	fm.sshConn.Close()
}

func handleFTP(conn Connection) {
	var fm ManagerFTP = ManagerFTP{}

	// Attempt to connect to the FTP server
	err := fm.connect(conn)
	if err != nil {
		logger.Debugf("Error connecting to FTP: %v\n", err)
		return
	}

	// Define the source folder for downloads
	var srcFolder string = conn.Path

	// Define the local directory for downloads
	localDir := path.Join(download_folder, conn.Name)
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		// Create the local directory if it does not exist
		err := os.MkdirAll(localDir, os.ModePerm)
		if err != nil {
			logger.Debugf("Error creating directory: %v\n", err)
			return
		}
	}

	// Start downloading files from the source folder to the local directory
	err = recursivelyDownloadFTP(srcFolder, localDir, conn.Depth, &fm, conn) // Adjust the depth as needed
	if err != nil {
		logger.Debugf("Error downloading files: %v\n", err)
	}

	fm.ftpConn.Quit()
}

func handleSFTP(conn Connection) {
	var fm ManagerSFTP = ManagerSFTP{}

	// Attempt to connect to the SFTP server
	err := fm.connectToSFTP(conn)
	if err != nil {
		logger.Debugf("Error connecting to SFTP: %v\n", err)
		return
	}

	// Define the source folder for downloads
	var srcFolder string = conn.Path

	// Define the local directory for downloads
	localDir := path.Join(download_folder, conn.Name)
	if _, err := os.Stat(localDir); os.IsNotExist(err) {
		// Create the local directory if it does not exist
		err := os.MkdirAll(localDir, os.ModePerm)
		if err != nil {
			logger.Debugf("Error creating directory: %v\n", err)
			return
		}
	}

	// Start downloading files from the source folder to the local directory
	err = recursivelyDownloadSFTP(srcFolder, localDir, conn.Depth, fm, conn)
	if err != nil {
		logger.Debugf("Error downloading files: %v\n", err)
	}

	// Ensure the SFTP client and SSH connection are closed when done
	fm.sftpClient.Close()
	fm.sshConn.Close()
}

func handleConnection(conns []Connection, split string) {
	for {
		for i, conn := range conns {
			logger.Debugf("=== Connection: %d of %d, Split: %s, Name: %s, Host: %s, Port: %d, Protocol: %s, Username: %s\n", i+1, len(conns), split, conn.Name, conn.Host, conn.Port, conn.Protocol, conn.Username)

			switch conn.Protocol {
			case "sftp":
				handleSFTP(conn)
			case "ftp":
				handleFTP(conn)
			case "ftpoverssh":
				handleFTPoverSSH(conn)
			}

			time.Sleep(time.Duration(conn.Delay) * time.Second)
		}
		time.Sleep(10 * time.Second)
	}
}

func (fm *ManagerFTPoverSSH) connect(conn Connection) error {
	var config *ssh.ClientConfig

	// SFTP connection logic
	logger.Debugf("Connecting to SFTP: Host: %s, Port: %d, Username: %s\n", conn.Host, conn.Port, conn.Username)
	// Check if the connection contains an SSH key
	if conn.SSHKeyPath != "" {
		logger.Infof("Using SSH key for authentication: %s\n", conn.SSHKeyPath)
		key, err := os.ReadFile(conn.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key file: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %v", err)
		}

		config = &ssh.ClientConfig{
			User: conn.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second, // Add timeout of 5 seconds
		}

	} else {
		logger.Infof("Using password for authentication: %s\n", conn.Username)
		// Set up SSH client configuration
		config = &ssh.ClientConfig{
			User: conn.Username,
			Auth: []ssh.AuthMethod{
				ssh.Password(conn.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second, // Add timeout of 5 seconds
		}
	}

	// Connect to the SSH server
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	sshConn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %v", err)
	}
	fm.sshConn = sshConn

	logger.Infof("Successfully connected to SSH: %s\n", conn.Name)

	// TODO: change default port for ftp connection into ssh tunnel
	// Create an FTP client over the SSH connection
	ftpConn, err := ftp.Dial("127.0.0.1:21", ftp.DialWithDialFunc(func(network, addr string) (net.Conn, error) {
		logger.Debugf("Dialing FTP over SSH: network=%s, addr=%s\n", network, addr)
		conn, err := fm.sshConn.Dial(network, addr)
		if err != nil {
			logger.Errorf("Failed to dial FTP over SSH: %v\n", err)
		}
		return conn, err
	}))
	if err != nil {
		return fmt.Errorf("failed to dial FTP over SSH: %v", err)
	}

	// Login to the FTP server
	err = ftpConn.Login(conn.Username, conn.Password)
	if err != nil {
		ftpConn.Quit()
		return fmt.Errorf("failed to login to FTP: %v", err)
	}

	fm.ftpConn = ftpConn
	logger.Debugf("Connected to FTP over SSH: %s\n", conn.Name)
	return nil
}

func (fm *ManagerFTP) connect(conn Connection) error {
	// FTP connection logic
	logger.Debugf("Connecting to FTP: Host: %s, Port: %d, Username: %s\n", conn.Host, conn.Port, conn.Username)

	// Set up FTP client configuration
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	ftpConn, err := ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("failed to dial FTP: %v", err)
	}

	//welcomeMessage, err := ftp.StatusText()

	// Login to the FTP server
	err = ftpConn.Login(conn.Username, conn.Password)
	if err != nil {
		ftpConn.Quit()
		return fmt.Errorf("failed to login to FTP: %v", err)
	}

	fm.ftpConn = ftpConn
	logger.Debugf("Connected to FTP: %s\n", conn.Name)
	return nil
}

func (fm *ManagerFTP) readDir(remotePath string) ([]*ftp.Entry, error) {
	// Read the directory contents from the remote FTP server
	entries, err := fm.ftpConn.List(remotePath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %v", err)
	}
	var fileStats []*ftp.Entry
	for _, entry := range entries {
		if entry.Type == ftp.EntryTypeFile {
			fileStats = append(fileStats, entry)
			logger.Debugf("File: %s, Size: %d, Modified: %s\n", entry.Name, entry.Size, entry.Time)
		}
	}
	return fileStats, nil
}

func (fm *ManagerFTP) downloadFile(remotePath, localPath string) error {
	// Open the remote file
	resp, err := fm.ftpConn.Retr(remotePath)
	if err != nil {
		return fmt.Errorf("error opening remote file: %v", err)
	}
	defer resp.Close()

	// Create the local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

	// Copy the file contents from the remote file to the local file
	_, err = io.Copy(localFile, resp)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	logger.Infof("Downloaded file: %s\n", remotePath)
	return nil
}

func (fm *ManagerFTP) deleteFile(remotePath string) error {
	// Delete the file from the FTP server
	err := fm.ftpConn.Delete(remotePath)
	if err != nil {
		return fmt.Errorf("error deleting file from FTP server: %v", err)
	}
	logger.Debugf("Deleted file from FTP server: %s\n", remotePath)
	return nil
}

func (fm *ManagerFTPoverSSH) readDir(remotePath string) ([]*ftp.Entry, error) {
	// Read the directory contents from the remote FTP server
	entries, err := fm.ftpConn.List(remotePath)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %v", err)
	}
	var fileStats []*ftp.Entry
	for _, entry := range entries {

		if entry.Type == ftp.EntryTypeFile {
			fileStats = append(fileStats, entry)
			logger.Debugf("File: %s, Size: %d, Modified: %s\n", entry.Name, entry.Size, entry.Time)
		} else if entry.Type == ftp.EntryTypeFolder {
			fileStats = append(fileStats, entry)
			logger.Debugf("Folder: %s, Size: %d, Modified: %s\n", entry.Name, entry.Size, entry.Time)
		} else {
			logger.Debugf("Other filetype: %s, Size: %d, Modified: %s\n", entry.Name, entry.Size, entry.Time)

		}

	}
	return fileStats, nil
}

func (fm *ManagerFTPoverSSH) downloadFile(remotePath, localPath string) error {
	// Open the remote file
	resp, err := fm.ftpConn.Retr(remotePath)
	if err != nil {
		return fmt.Errorf("error opening remote file: %v", err)
	}
	defer resp.Close()

	// Create the local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("error creating local file: %v", err)
	}
	defer localFile.Close()

	// Copy the file contents from the remote file to the local file
	_, err = io.Copy(localFile, resp)
	if err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}

	logger.Infof("Downloaded file: %s\n", remotePath)
	return nil
}

func (fm *ManagerFTPoverSSH) deleteFile(remotePath string) error {
	// Delete the file from the FTP server
	err := fm.ftpConn.Delete(remotePath)
	if err != nil {
		return fmt.Errorf("error deleting file from FTP server: %v", err)
	}
	logger.Debugf("Deleted file from FTP server: %s\n", remotePath)
	return nil
}

func (fm *ManagerSFTP) connectToSFTP(conn Connection) error {

	var config *ssh.ClientConfig

	// SFTP connection logic
	logger.Debugf("Connecting to SFTP: Host: %s, Port: %d, Username: %s\n", conn.Host, conn.Port, conn.Username)
	// Check if the connection contains an SSH key
	if conn.SSHKeyPath != "" {
		logger.Infof("Using SSH key for authentication: %s\n", conn.SSHKeyPath)
		key, err := os.ReadFile(conn.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key file: %v", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %v", err)
		}

		config = &ssh.ClientConfig{
			User: conn.Username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second, // Add timeout of 5 seconds
		}

	} else {
		logger.Infof("Using password for authentication: %s\n", conn.Username)
		// Set up SSH client configuration
		config = &ssh.ClientConfig{
			User: conn.Username,
			Auth: []ssh.AuthMethod{
				ssh.Password(conn.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         5 * time.Second, // Add timeout of 5 seconds
		}
	}

	// Connect to the SSH server
	addr := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	sshConn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %v", err)
	}
	fm.sshConn = sshConn
	// Create an SFTP client
	sftpClient, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return fmt.Errorf("failed to create SFTP client: %v", err)
	}
	fm.sftpClient = sftpClient
	// Get server version
	serverVersion := fm.sshConn.ServerVersion()
	logger.Debugf("Connected to SFTP: %s and version: %s\n", conn.Name, serverVersion)
	return nil
}

func recreateFolder(folderPath string) error {
	// Delete the folder and its contents
	err := os.RemoveAll(folderPath)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %v", err)
	}

	// Create the folder again
	err = os.MkdirAll(folderPath, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create folder: %v", err)
	}

	logger.Debugf("Folder recreated successfully: %s\n", folderPath)
	return nil
}

func checkHostPort(host string, port int) bool {
	timeout := 2 * time.Second
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func generateSSHKeysForConnections(config Config, keysDir string) error {
	// Create the keys directory if it doesn't exist
	err := os.MkdirAll(keysDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create keys directory: %v", err)
	}

	// Loop through each connection name and generate an SSH key pair
	for _, conn := range config.Connections {
		keyPath := fmt.Sprintf("%s/%s", keysDir, conn.Name)
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "")
			err := cmd.Run()
			if err != nil {
				return fmt.Errorf("failed to generate SSH key pair for connection %s: %v", conn.Name, err)
			}
			logger.Infof("SSH key pair generated for connection: %s", conn.Name)
		} else {
			logger.Infof("SSH key pair already exists for connection: %s", conn.Name)
		}
	}

	return nil
}

func greeting() {
	logger.Infof("")
	logger.Infof("  __ _                        __         ")
	logger.Infof(" / _| |                      / _|          ")
	logger.Infof("| |_| |_ _ __ __ _ _ __  ___| |_ ___ _ __ ")
	logger.Infof("|  _| __| '__/ _` | '_ \\/ __|  _/ _ \\ '__|")
	logger.Infof("| | | |_| | | (_| | | | \\__ \\ ||  __/ |   ")
	logger.Infof("|_|  \\__|_|  \\__,_|_| |_|___/_| \\___|_|   ")
	logger.Infof("")
}

func main() {

	greeting()
	//if err := sendEmail("redfosforjs@gmail.com", "Some text", "Some text"); err != nil {
	//  logger.Fatalf("Failed to send email: %v", err)
	//}

	port := flag.Int("port", 8080, "Port for the HTTP server")
	download := flag.String("download", "download", "Directory for storing downloaded files")
	threads := flag.Int("threads", 5, "Number of concurrent threads to use")
	truncate := flag.Bool("truncate", false, "Whether to truncate the database before starting")
	clean := flag.Bool("clean", false, "Whether to clean the download folder before starting")
	debug := flag.Bool("debug", false, "Enable debug mode")
	keygen := flag.Bool("keygen", false, "Generate SSH keys for connections")

	flag.Parse()

	download_folder = *download

	var err error

	// Open a file for writing logs
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	// Set the output of the logger to the file
	if *debug {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetOutput(file)
		logger.SetLevel(logrus.InfoLevel)

	}

	// Log the version informatXzXion
	logger.Infof("App Version: %s, Build Time: %s", version, buildTime)

	// Set the log level (optional, default is InfoLevel)

	config, err := readConfig("connections.yaml")
	if err != nil {
		logger.Fatalf("Error: %v\n", err)
		return
	}

	if *keygen {

		if err := generateSSHKeysForConnections(config, "keys"); err != nil {
			logger.Fatalf("Failed to generate SSH keys: %v", err)
		}
		return
	}

	db.conn, err = openDatabase()
	if err != nil {
		logger.Fatalf("Failed to open database: %v", err)
	}
	defer db.conn.Close()

	err = createTable(db.conn)
	if err != nil {
		logger.Fatalf("Failed to create table: %v", err)
	}

	logger.Debug("Database setup completed successfully.")

	if *clean {
		recreateFolder(download_folder)
	}

	if *truncate {
		logger.Debug("Truncating the database as per the flag.")
		if err := truncateDatabase(db.conn); err != nil {
			logger.Fatalf("Failed to truncate database: %v", err)
		}
	}

	go handleHTTP(*port)

	Connections = splitConnections(config, *threads)

	logger.Debug("Splitted Connections Table:")
	logger.Infof("-------------------------------------------------------------------------")
	logger.Infof("| Group     | Name          | Host       | Port | Protocol | Username   |")
	logger.Infof("-------------------------------------------------------------------------")
	for split, conns := range Connections.Connection {
		for _, conn := range conns {
			logger.Infof("| %-9s | %-13s | %-10s | %-4d | %-8s | %-10s |", split, conn.Name, conn.Host, conn.Port, conn.Protocol, conn.Username)
		}
	}
	logger.Infof("-------------------------------------------------------------------------")

	for split, conns := range Connections.Connection {
		logger.Infof("=== %s - %d ===\n", split, len(conns))
		for i := range conns {
			if checkHostPort(conns[i].Host, conns[i].Port) {
				conns[i].Status = true
				logger.Infof("Connection to %s:%d is available\n", conns[i].Host, conns[i].Port)
			} else {
				conns[i].Status = false
				logger.Errorf("Connection to %s:%d is not available\n", conns[i].Host, conns[i].Port)
			}
		}

		go handleConnection(conns, split)
	}

	// Block main goroutine until an interrupt signal is received
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Println("Shutting down application...")

	// Close the database connection
	if db.conn != nil {
		db.conn.Close()
	}
	// Perform any additional cleanup if necessary
	// ...

	logger.Println("Application exiting")
}
