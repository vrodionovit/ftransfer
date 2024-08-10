#!/bin/bash


# Check if the system is Debian-based
# Check if the file /etc/debian_version exists, which indicates a Debian-based system.
# If the file does not exist, print a message and exit the script with a status of 1.
# if ! [ -f /etc/debian_version ]; then
#     echo "This script is intended for Debian-based systems only."
#     exit 1
# fi

# Check if the system is Debian-based using /etc/os-release
if ! grep -q "ID=debian" /etc/os-release && ! grep -q "ID_LIKE=debian" /etc/os-release; then
    echo "This script is intended for Debian-based systems only."
    exit 1
fi


# Check if OpenSSH server is already installed, if not, install it
if ! dpkg -l | grep -q openssh-server; then
    sudo apt-get update
    sudo apt-get install -y openssh-server
else
    echo "OpenSSH server is already installed."
fi

# Create a new group for SFTP users
sudo groupadd sftpusers

# Create a new user and add to the SFTP group
read -p "Enter the username for the new SFTP user: " username

# Check if the user already exists
if id "$username" &>/dev/null; then
    echo "User $username already exists."
    exit 1
fi




sudo useradd -m -G sftpusers -s /sbin/nologin $username

# Ask user for a password
read -sp "Enter the password for the new SFTP user: " password

# Set password for the new user
echo "$username:$password" | sudo chpasswd

# Create the SFTP directory structure
sudo mkdir -p /sftpdata/$username/upload
sudo chown root:root /sftpdata/$username
sudo chmod 755 /sftpdata/$username
sudo chown $username:sftpusers /sftpdata/$username/upload


# Create the .ssh directory and authorized_keys file
sudo -u $username mkdir -p /home/$username/.ssh
sudo -u $username touch /home/$username/.ssh/authorized_keys

# Set the appropriate permissions
sudo -u $username chmod 700 /home/$username/.ssh
sudo -u $username chmod 600 /home/$username/.ssh/authorized_keys


# Make a backup of the current SSH configuration
sudo cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak

# Configure SSH for SFTP
sudo bash -c 'cat >> /etc/ssh/sshd_config <<EOF

# SFTP configuration
Match Group sftpusers
    ChrootDirectory /sftpdata/%u
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
EOF'

# Restart the SSH service to apply changes
sudo systemctl restart sshd

# Check if the firewall is active
if sudo ufw status | grep -q "Status: active"; then
    echo "Firewall is active."

    read -p "Do you want to open port 22 for SFTP connections? (yes/no): " answer
    if [[ $answer == "yes" ]]; then
        echo "Opening port 22 for SFTP connections..."
        # Allow SSH connections through the firewall
        sudo ufw allow OpenSSH

        # Allow SFTP connections through the firewall
        sudo ufw allow 22/tcp

        # Reload the firewall to apply changes
        sudo ufw reload
    else
        echo "Port 22 will not be opened for SFTP connections."
    fi
    
else
    echo "Firewall is not active."
fi




echo "SFTP server setup is complete. User $username can now connect to the server."
