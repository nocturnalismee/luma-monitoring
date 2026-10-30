#!/bin/bash
#
#
#	LUMA - Lightweight Server Monitoring Agent - Install Script
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
umask 077

# Prefer IPv4 when fetching from GitHub, fallback to IPv6 if needed
github_wget() {
	local url=${!#}
	if ! wget -4 "$@"; then
		echo "IPv4 request failed for $url, retrying with IPv6..."
		if ! wget -6 "$@"; then
			echo "ERROR: Unable to fetch $url via IPv4 or IPv6." >&2
			return 1
		fi
	fi
	return 0
}

# Branch
BRANCH="master"

# Check if first argument is branch or SID
if [ ${#1} -ne 32 ]
then
	BRANCH=$1
	shift
fi

# Check if install script is run by root
echo "Checking root privileges..."
if [ "$EUID" -ne 0 ]
	then echo "ERROR: Please run the install script as root."
	exit 1
fi
echo "... done."

apply_agent_permissions() {
	local run_user=$1

	if [ "$run_user" = "root" ]
	then
		chown -R root:root /etc/luma || return 1
	else
		chown -R luma:luma /etc/luma || return 1
	fi

	chmod 700 /etc/luma || return 1
	chmod 700 /etc/luma/luma_agent.sh || return 1
	chmod 600 /etc/luma/luma.cfg || return 1
}

# Check if the selected branch exists
if github_wget --spider -q https://raw.githubusercontent.com/luma-monitoring/agent/$BRANCH/luma_agent.sh
then
	echo "Installing from $BRANCH branch..."
else
	echo "ERROR: Branch $BRANCH does not exist." >&2
	exit 1
fi

# Fetch Server Unique ID
SID=$1

# Make sure SID is not empty
echo "Checking Server ID (SID)..."
if [ -z "$SID" ]
	then echo "ERROR: First parameter missing."
	exit
fi
echo "... done."

# Check if user has selected to run agent as 'root' or as 'luma' user
if [ -z "$2" ]
	then echo "ERROR: Second parameter missing."
	exit
fi
case "$2" in
	1)
		RUN_USER=root
		;;
	0)
		RUN_USER=luma
		;;
	*)
		echo "ERROR: Second parameter must be 0 for 'luma' or 1 for 'root'." >&2
		exit 1
		;;
esac

# Check for wget and cron/systemd availability
echo "Checking system utilities..."
command -v wget >/dev/null 2>&1 || { echo "ERROR: wget is required to run this agent." >&2; exit 1; }
USE_CRON=0
USE_SYSTEMD=0
SYSTEMCTL_AVAILABLE=0
EXISTING_CRON=0
CRON_ACTIVE=0
if command -v systemctl >/dev/null 2>&1; then
	if [ -d /run/systemd/system ] || systemctl list-units >/dev/null 2>&1; then
		SYSTEMCTL_AVAILABLE=1
	fi
fi
if command -v crontab >/dev/null 2>&1; then
	if crontab -u root -l 2>/dev/null | grep -q 'luma_agent.sh'; then
		EXISTING_CRON=1
	elif id -u luma >/dev/null 2>&1 && crontab -u luma -l 2>/dev/null | grep -q 'luma_agent.sh'; then
		EXISTING_CRON=1
	fi
	CRON_ACTIVE=$EXISTING_CRON
	if command -v pgrep >/dev/null 2>&1 && [ "$CRON_ACTIVE" -ne 1 ]; then
		for cron_process in cron crond cronie systemd-cron fcron busybox-cron busybox-crond; do
			if pgrep -x "$cron_process" >/dev/null 2>&1 || pgrep -f "$cron_process" >/dev/null 2>&1; then
				CRON_ACTIVE=1
				break
			fi
		done
	fi
	if [ "$CRON_ACTIVE" -ne 1 ] && [ "$SYSTEMCTL_AVAILABLE" -eq 1 ]; then
		for cron_service in cron crond cronie systemd-cron fcron busybox-cron busybox-crond; do
			if systemctl is-active --quiet "$cron_service"; then
				CRON_ACTIVE=1
				break
			fi
		done
		if [ "$CRON_ACTIVE" -ne 1 ]; then
			if systemctl list-units --type=service --state=active 2>/dev/null | grep -Ei '\bcron(ie)?\b' >/dev/null 2>&1; then
				CRON_ACTIVE=1
			fi
		fi
	fi
	if [ "$CRON_ACTIVE" -ne 1 ] && [ "$SYSTEMCTL_AVAILABLE" -ne 1 ]; then
		CRON_ACTIVE=1
	fi
	if [ "$CRON_ACTIVE" -eq 1 ]; then
		USE_CRON=1
	fi
fi
if [ "$USE_CRON" -ne 1 ] && [ "$SYSTEMCTL_AVAILABLE" -eq 1 ] && command -v systemd-run >/dev/null 2>&1; then
	USE_SYSTEMD=1
fi
if [ "$USE_CRON" -ne 1 ] && [ "$USE_SYSTEMD" -ne 1 ]; then
	echo "ERROR: Neither cron nor systemd with systemd-run is available to schedule the agent." >&2
	exit 1
fi
echo "... done."

# Remove old agent (if exists)
echo "Checking if there's any old luma agent already installed..."
if [ -d /etc/luma ]
then
	echo "Old luma agent found, deleting it..."
	rm -rf /etc/luma
else
	echo "No old luma agent found..."
fi
echo "... done."

# Creating agent folder
echo "Creating the luma agent folder..."
mkdir -p /etc/luma
echo "... done."

# Fetching the agent
echo "Fetching the agent..."
if ! github_wget -t 1 -T 30 -qO /etc/luma/luma_agent.sh https://raw.githubusercontent.com/luma-monitoring/agent/$BRANCH/luma_agent.sh
then
	echo "ERROR: Failed to download the agent script from GitHub." >&2
	exit 1
fi
echo "... done."

# Fetching the config file
echo "Fetching the config file..."
if ! github_wget -t 1 -T 30 -qO /etc/luma/luma.cfg https://raw.githubusercontent.com/luma-monitoring/agent/$BRANCH/luma.cfg
then
	echo "ERROR: Failed to download the agent configuration from GitHub." >&2
	exit 1
fi
echo "... done."

# Inserting Server ID (SID) into the agent config
echo "Inserting Server ID (SID) into agent config..."
sed -i "s/SID=\"\"/SID=\"$SID\"/" /etc/luma/luma.cfg
echo "... done."

# Killing any running luma agents
echo "Making sure no luma agent scripts are currently running..."
ps aux | grep -ie luma_agent.sh | awk '{print $2}' | xargs -r kill -9
echo "... done."

# Checking if luma user exists
echo "Checking if luma user already exists..."
if id -u luma >/dev/null 2>&1
then
	echo "The luma user already exists, killing its processes..."
	pkill -9 -u `id -u luma`
	echo "Deleting luma user..."
	userdel luma
	echo "Creating the new luma user..."
	useradd luma -r -d /etc/luma -s /bin/false
else
	echo "The luma user doesn't exist, creating it now..."
	useradd luma -r -d /etc/luma -s /bin/false
fi
echo "Assigning permissions for the selected runtime user..."
if ! apply_agent_permissions "$RUN_USER"
then
	echo "ERROR: Failed to assign permissions for the agent files." >&2
	exit 1
fi
echo "... done."

# Removing old cronjob (if exists)
echo "Removing any old luma cronjob, if exists..."
if command -v crontab >/dev/null 2>&1
then
	crontab -u root -l 2>/dev/null | grep -v 'luma_agent.sh'  | crontab -u root - >/dev/null 2>&1
	crontab -u luma -l 2>/dev/null | grep -v 'luma_agent.sh'  | crontab -u luma - >/dev/null 2>&1
fi
echo "... done."

# Removing old systemd service/timer (if exists)
if [ "$SYSTEMCTL_AVAILABLE" -eq 1 ]; then
	systemctl stop luma_agent.timer >/dev/null 2>&1
	systemctl disable luma_agent.timer >/dev/null 2>&1
	systemctl stop luma_agent.service >/dev/null 2>&1
	systemctl disable luma_agent.service >/dev/null 2>&1
	systemctl daemon-reload >/dev/null 2>&1
fi
rm -f /etc/systemd/system/luma_agent.timer >/dev/null 2>&1
rm -f /etc/systemd/system/luma_agent.service >/dev/null 2>&1
rm -f /etc/luma/luma_systemd_launcher.sh >/dev/null 2>&1
rm -f /usr/local/sbin/luma_systemd_launcher.sh >/dev/null 2>&1

# Setup the new systemd or cronjob timer to run the agent every minute
if [ "$USE_CRON" -eq 1 ]
then
	# Default is running the agent as 'luma' user, unless chosen otherwise by the client when fetching the installation code from the luma website.
	if [ "$RUN_USER" = "root" ]
	then
		echo "Setting up the new cronjob as 'root' user..."
		crontab -u root -l 2>/dev/null | { cat; echo "* * * * * bash /etc/luma/luma_agent.sh >> /etc/luma/luma_cron.log 2>&1"; } | crontab -u root - >/dev/null 2>&1
	else
		echo "Setting up the new cronjob as 'luma' user..."
		crontab -u luma -l 2>/dev/null | { cat; echo "* * * * * bash /etc/luma/luma_agent.sh >> /etc/luma/luma_cron.log 2>&1"; } | crontab -u luma - >/dev/null 2>&1
	fi
elif [ "$USE_SYSTEMD" -eq 1 ]
then
	echo "Setting up systemd timer..."
	SERVICE_USER=$RUN_USER
	cat > /usr/local/sbin/luma_systemd_launcher.sh <<-'EOF'
#!/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

ServiceUser=$1
if [ "$ServiceUser" != "root" ] && [ "$ServiceUser" != "luma" ]
then
	ServiceUser=root
fi

if ! command -v systemd-run >/dev/null 2>&1
then
	echo "ERROR: systemd-run is required for systemd timer scheduling." >&2
	exit 1
fi

RunID=$(date +%s)-$$
UnitName="luma_agent_${RunID}"
AgentCommand='exec /bin/bash /etc/luma/luma_agent.sh >> /etc/luma/luma_cron.log 2>&1'

if [ "$ServiceUser" = "root" ]
then
	systemd-run --quiet --collect --unit="$UnitName" /bin/bash -c "$AgentCommand"
else
	systemd-run --quiet --collect --unit="$UnitName" --property="User=$ServiceUser" /bin/bash -c "$AgentCommand"
fi
EOF
	chown root:root /usr/local/sbin/luma_systemd_launcher.sh >/dev/null 2>&1
	chmod 700 /usr/local/sbin/luma_systemd_launcher.sh
	cat > /etc/systemd/system/luma_agent.service <<-EOF
[Unit]
Description=luma Agent Launcher

[Service]
Type=oneshot
ExecStart=/bin/bash /usr/local/sbin/luma_systemd_launcher.sh $SERVICE_USER
EOF
	cat > /etc/systemd/system/luma_agent.timer <<-EOF
[Unit]
Description=Runs luma agent every minute

[Timer]
OnBootSec=1min
OnCalendar=*-*-* *:*:00 UTC
AccuracySec=1s
RandomizedDelaySec=0
Persistent=true
Unit=luma_agent.service

[Install]
WantedBy=timers.target
EOF
	systemctl daemon-reload >/dev/null 2>&1
	systemctl enable --now luma_agent.timer >/dev/null 2>&1
	systemctl restart luma_agent.timer >/dev/null 2>&1
else
	echo "ERROR: Unable to configure scheduling for the agent." >&2
	exit 1
fi
echo "... done."

# Cleaning up install file
echo "Cleaning up the installation file..."
if [ -f $0 ]
then
	rm -f $0
fi
echo "... done."

# Let luma platform know install has been completed
echo "Letting luma platform know the installation has been completed..."
POST="v=install&s=$SID"
wget -t 1 -T 30 -qO- --post-data "$POST" https://sm.luma.local/ &> /dev/null
echo "... done."

# Start the agent
if [ "$RUN_USER" = "root" ]
then
	echo "Starting the agent under the 'root' user..."
	bash /etc/luma/luma_agent.sh > /dev/null 2>&1 &
else
	echo "Starting the agent under the 'luma' user..."
	sudo -u luma bash /etc/luma/luma_agent.sh > /dev/null 2>&1 &
fi
echo "... done."

# All done
echo "luma agent installation completed."
