#!/bin/bash
#
#
#	LUMA - Lightweight Server Monitoring Agent - Uninstall Script
#	Copyright 2015 - 2026 @  LUMA
#
#
#		DISCLAIMER OF WARRANTY
#
#	The Software is provided "AS IS" and "WITH ALL FAULTS," without warranty of any kind, 
#	including without limitation the warranties of merchantability, fitness for a particular purpose and non-infringement. 
#	LUMA makes no warranty that the Software is free of defects or is suitable for any particular purpose. 
#	In no event shall LUMA be responsible for loss or damages arising from the installation or use of the Software, 
#	including but not limited to any indirect, punitive, special, incidental or consequential damages of any character including, 
#	without limitation, damages for loss of goodwill, work stoppage, computer failure or malfunction, or any and all other commercial damages or losses. 
#	The entire risk as to the quality and performance of the Software is borne by you, the user.
#
#

# Set PATH
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

# Check if uninstall script is run by root
echo "Checking root privileges..."
if [ "$EUID" -ne 0 ]
  then echo "ERROR: Please run the uninstall script as root."
  exit
fi
echo "... done."

# Fetch Server Unique ID
SID=$1

# Remove old agent (if exists)
echo "Checking if luma agent folder exists..."
if [ -d /etc/luma ]
then
	echo "Old luma agent found, deleting it..."
	rm -rf /etc/luma
else
	echo "No old luma agent folder found..."
fi
echo "... done."

# Killing any running luma agents
echo "Killing any luma agent scripts that may be currently running..."
ps aux | grep -ie luma_agent.sh | awk '{print $2}' | xargs -r kill -9
echo "... done."

# Checking if luma user exists
echo "Checking if luma user exists..."
if id -u luma >/dev/null 2>&1
then
	echo "The luma user exists, killing its processes..."
	pkill -9 -u `id -u luma`
	echo "Deleting luma user..."
	userdel luma
else
	echo "The luma user doesn't exist..."
fi
echo "... done."

# Removing systemd service/timer if exists
echo "Removing any luma systemd service/timer, if exists..."
if command -v systemctl >/dev/null 2>&1
then
	systemctl stop luma_agent.timer >/dev/null 2>&1
	systemctl disable luma_agent.timer >/dev/null 2>&1
	rm -f /etc/systemd/system/luma_agent.timer
	systemctl stop luma_agent.service >/dev/null 2>&1
	systemctl disable luma_agent.service >/dev/null 2>&1
	rm -f /etc/systemd/system/luma_agent.service
	systemctl daemon-reload >/dev/null 2>&1
fi
rm -f /usr/local/sbin/luma_systemd_launcher.sh >/dev/null 2>&1
echo "... done."

# Removing cronjob (if exists)
echo "Removing any luma cronjob, if exists..."
if command -v crontab >/dev/null 2>&1
then
    crontab -u root -l 2>/dev/null | grep -v 'luma_agent.sh'  | crontab -u root - >/dev/null 2>&1
    crontab -u luma -l 2>/dev/null | grep -v 'luma_agent.sh'  | crontab -u luma - >/dev/null 2>&1
fi
echo "... done."

# Cleaning up uninstall file
echo "Cleaning up the uninstall file..."
if [ -f $0 ]
then
    rm -f $0
fi
echo "... done."

# Let luma platform know uninstall has been completed
echo "Letting luma platform know the uninstallation has been completed..."
POST="v=uninstall&s=$SID"
wget -t 1 -T 30 -qO- --post-data "$POST" https://sm.luma.local/ &> /dev/null
echo "... done."

# All done
echo "luma agent uninstallation completed."
