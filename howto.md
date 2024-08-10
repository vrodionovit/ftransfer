# To create a simple SFTP server and set it up on both Linux and Windows, follow the steps below:

### Setting Up SFTP Server on Linux

1. **Install OpenSSH Server:**
   OpenSSH is a widely used SSH server that includes SFTP functionality. Install it using the package manager for your Linux distribution.

   For Debian/Ubuntu:
   ```sh
   sudo apt update
   sudo apt install openssh-server
   ```

   For CentOS/RHEL:
   ```sh
   sudo yum install openssh-server
   ```

2. **Configure OpenSSH:**
   Edit the SSH configuration file to enable SFTP and configure the necessary settings.

   ```sh
   sudo nano /etc/ssh/sshd_config
   ```

   Ensure the following lines are present and uncommented:
   ```
   Subsystem sftp /usr/lib/openssh/sftp-server
   ```

   Optionally, you can configure a specific directory for SFTP users:
   ```
   Match User sftpuser
       ChrootDirectory /home/sftpuser
       ForceCommand internal-sftp
       AllowTcpForwarding no
       X11Forwarding no
   ```

3. **Create SFTP User:**
   Create a new user for SFTP access and set up the directory structure.

   ```sh
   sudo adduser sftpuser
   sudo mkdir -p /home/sftpuser/uploads
   sudo chown root:root /home/sftpuser
   sudo chmod 755 /home/sftpuser
   sudo chown sftpuser:sftpuser /home/sftpuser/uploads
   ```

4. **Restart SSH Service:**
   Restart the SSH service to apply the changes.

   ```sh
   sudo systemctl restart ssh
   ```

### Setting Up SFTP Server on Windows

1. **Install OpenSSH Server:**
   Windows 10 and Windows Server 2019 include OpenSSH as an optional feature. Install it using the following steps:

   - Open **Settings**.
   - Go to **Apps** > **Optional Features**.
   - Click **Add a feature**.
   - Find and install **OpenSSH Server**.

2. **Configure OpenSSH:**
   Open the SSH configuration file to enable SFTP and configure the necessary settings.

   ```sh
   notepad C:\ProgramData\ssh\sshd_config
   ```

   Ensure the following lines are present and uncommented:
   ```
   Subsystem sftp sftp-server.exe
   ```

   Optionally, you can configure a specific directory for SFTP users:
   ```
   Match User sftpuser
       ChrootDirectory C:\sftp\sftpuser
       ForceCommand internal-sftp
       AllowTcpForwarding no
       X11Forwarding no
   ```

3. **Create SFTP User:**
   Create a new user for SFTP access and set up the directory structure.

   ```sh
   net user sftpuser /add
   mkdir C:\sftp\sftpuser\uploads
   icacls C:\sftp\sftpuser /grant Administrators:F /t
   icacls C:\sftp\sftpuser /grant sftpuser:RX /t
   ```

4. **Restart SSH Service:**
   Restart the SSH service to apply the changes.

   ```sh
   Restart-Service sshd
   ```

### Connecting to the SFTP Server

You can use any SFTP client to connect to the SFTP server. Popular clients include:

- **FileZilla**: A free, open-source FTP client.
- **WinSCP**: A popular SFTP and FTP client for Windows.
- **Cyberduck**: A libre server and cloud storage browser for Mac and Windows.

Use the SFTP user credentials created in the steps above to connect to the server.
*/


### Creating SSH Key Pair for SFTP

To enhance security, you can use SSH key-based authentication for SFTP access. Follow these steps to create an SSH key pair and configure the SFTP server to use it.

1. **Generate SSH Key Pair:**
   Use the `ssh-keygen` tool to generate a new SSH key pair.

   ```sh
   ssh-keygen -t rsa -b 2048 -f C:\Users\yourusername\.ssh\sftp_rsa
   ```

   This command will generate two files:
   - `sftp_rsa`: The private key (keep this secure and do not share it).
   - `sftp_rsa.pub`: The public key (this will be copied to the server).

2. **Copy Public Key to Server:**
   Copy the contents of the public key file (`sftp_rsa.pub`) to the server and add it to the `authorized_keys` file for the SFTP user.

   ```sh
   type C:\Users\yourusername\.ssh\sftp_rsa.pub | ssh sftpuser@yourserver "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"
   ```

3. **Set Permissions on Server:**
   Ensure the correct permissions are set for the `.ssh` directory and the `authorized_keys` file on the server.

   ```sh
   ssh sftpuser@yourserver "chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys"
   ```

4. **Configure SSH Daemon:**
   Ensure the SSH daemon on the server is configured to allow key-based authentication. Open the SSH configuration file and ensure the following lines are present and uncommented:

   ```sh
   notepad C:\ProgramData\ssh\sshd_config
   ```

   ```
   PubkeyAuthentication yes
   AuthorizedKeysFile .ssh/authorized_keys
   ```

5. **Restart SSH Service:**
   Restart the SSH service to apply the changes.

   ```sh
   Restart-Service sshd
   ```

### Connecting to the SFTP Server Using SSH Key

You can use any SFTP client to connect to the SFTP server using the SSH key. Here are examples for popular clients:

1. **FileZilla:**
   - Open FileZilla and go to `Edit` > `Settings`.
   - Under `Connection`, select `SFTP`.
   - Click `Add key file...` and select your private key file (`sftp_rsa`).
   - Connect to the server using the SFTP user credentials.

2. **WinSCP:**
   - Open WinSCP and create a new site.
   - Set the `File protocol` to `SFTP`.
   - Enter the hostname, port number, and username.
   - Click `Advanced...` and go to `SSH` > `Authentication`.
   - In the `Private key file` box, select your private key file (`sftp_rsa`).
   - Save the session and connect to the server.

3. **Cyberduck:**
   - Open Cyberduck and click `Open Connection`.
   - Set the `File protocol` to `SFTP (SSH File Transfer Protocol)`.
   - Enter the server address, port number, and username.
   - Click the `More Options` dropdown and select `Use Public Key Authentication`.
   - Choose your private key file (`sftp_rsa`).
   - Connect to the server.

By following these steps, you can securely connect to your SFTP server using SSH key-based authentication.


### Installing and Setting Up an FTP Server on Debian

To install and set up an FTP server on a Debian-based system, follow these steps:

1. **Update the Package List:**
   Update the package list to ensure you have the latest information on available packages.

   ```sh
   sudo apt-get update
   ```

2. **Install vsftpd:**
   Install the `vsftpd` package, which is a popular FTP server for Unix-like systems.

   ```sh
   sudo apt-get install -y vsftpd
   ```

3. **Configure vsftpd:**
   Open the vsftpd configuration file in a text editor.

   ```sh
   sudo nano /etc/vsftpd.conf
   ```

   Make the following changes to the configuration file:

   - Uncomment the following lines to allow local users to log in and enable write permissions:

     ```
     local_enable=YES
     write_enable=YES
     ```

   - Uncomment the following line to enable chroot for local users, which restricts them to their home directories:

     ```
     chroot_local_user=YES
     ```

   - Add the following line to allow passive mode connections (adjust the port range as needed):

     ```
     pasv_min_port=10000
     pasv_max_port=10100
     ```

   - Add the following line to specify the FTP banner message:

     ```
     ftpd_banner=Welcome to the FTP server.
     ```

4. **Restart vsftpd Service:**
   Restart the vsftpd service to apply the changes.

   ```sh
   sudo systemctl restart vsftpd
   ```

5. **Create FTP User:**
   Create a new user for FTP access.

   ```sh
   sudo adduser ftpuser
   ```

   Follow the prompts to set the password and other user details.

6. **Set Directory Permissions:**
   Set the appropriate permissions for the FTP user's home directory.

   ```sh
   sudo chown ftpuser:ftpuser /home/ftpuser
   sudo chmod 755 /home/ftpuser
   ```

7. **Allow FTP Through Firewall:**
   If you have a firewall enabled, allow FTP traffic through the firewall.

   ```sh
   sudo ufw allow 20/tcp
   sudo ufw allow 21/tcp
   sudo ufw allow 10000:10100/tcp
   sudo ufw reload
   ```

By following these steps, you can install and set up an FTP server on a Debian-based system. You can now connect to the FTP server using an FTP client with the credentials of the user you created.
