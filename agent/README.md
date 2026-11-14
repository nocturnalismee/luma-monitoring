# LUMA - Lightweight Server Monitoring Agent


## Provenance and repository notes (LUMA Monitoring)

This directory holds the agent files used by the LUMA Monitoring backend. It is
maintained as part of the `luma-monitoring` repository, not as an independent
upstream project.

- **Files in this directory** (`luma_agent.sh`, `luma.cfg`, `luma_install.sh`,
  `luma_update.sh`, `luma_uninstall.sh`) are copied verbatim from the upstream
  agent distribution, with the single exception listed under "Modifications".
  This copy corresponds to **agent version 2.4.2**, declared at line 24 of
  `luma_agent.sh` (`Version="2.4.2"`).
- **Modifications**: only URL placeholders were introduced, exactly as specified
  in `docs/DEPLOYMENT.md` section 4.2 (task `T-001`). No logic, payload format,
  or behaviour was changed. Nine URL literals were replaced by the placeholder:
  - host-based ingest/event URLs (2 places in `luma_agent.sh`, 1 place each in
    `luma_install.sh` and `luma_uninstall.sh`)
  - raw-source download URLs, including their repository/branch path segment
    (3 places in `luma_install.sh`, 2 places in `luma_update.sh`)
  The two literal strings that used to appear here are intentionally not repeated
  in this file, so that the acceptance check for `T-001` (`grep` over `agent/`)
  stays clean.
  The backend replaces `__LUMA_BASE_URL__` with the configured `LUMA_BASE_URL`
  when serving these files under `/agent/*` (see `docs/DEPLOYMENT.md` 4.1/4.3).
  The `$BRANCH` argument parsed by the installer/updater no longer affects any
  URL; it is retained for backwards compatibility only.
- **Copyright / warranty headers** at the top of every script are kept unchanged.
  Whether the upstream license permits modification, rebranding, and redistribution —
  and whether attribution must be preserved — is still an open question tracked as
  `Q-001` in `docs/DECISIONS.md`. Until that is answered, the interim default applies:
  keep all existing copyright/disclaimer headers intact.
- **Do not edit** these scripts beyond the placeholder changes above. Any further
  change requires a task entry in `docs/TASKS.md` and an update to `docs/PROTOCOL.md`.
- **Known cosmetic leftovers, deliberately untouched by `T-001`** (out of scope for a
  URL-only change; fix them only via a new task): comments and user-facing strings still
  refer to GitHub and to `$BRANCH` — `luma_install.sh:24`, `luma_install.sh:73`
  ("Installing from $BRANCH branch..."), `luma_install.sh:185`, `luma_install.sh:194`,
  `luma_update.sh:24`, `luma_update.sh:485`, `luma_update.sh:659`. The fetches now target
  this backend under `/agent/*`, not GitHub, so those messages are misleading but harmless.


Documentation available here: TODO

### Changelog

#### Version 2.4.2:
- Added drive health monitoring support for SAS drives
- Improved MegaRAID discovery
- Fixed missing (in some cases) drive-health entries for additional disks on hardware RAID systems
- Reduced repeated RAID probing and duplicate drive-health entries
- Fixed malformed (in some cases) drive-health payloads

#### Version 2.4.1:
- Improved the install and update scripts
- Added the `CheckReboot` configuration
- Disabled `needs-restarting` checking on non-sqlite or unknown RPMDB backends
- `needs-restarting` on sqlite-backed RPMDB systems is now performed once per hour
- Harden file permissions and config variables
- Optimize performance

#### Version 2.4.0:
- Added support for Outgoing Pings on defined ports
- Improved RAM metrics collection
- Improved service status monitoring
- Improved disk metrics collection
- Improved update method

#### Version 2.3.8:
- Fixed an issue where, in some cases, IO metrics were not measured correctly

#### Version 2.3.7:
- Further improvements on the agent's running schedule
- Debug Mode improvements.

#### Version 2.3.6:
- Improved thin LVM monitoring to detect underlying RAID (if exists)

#### Version 2.3.5:
- Improved disk monitoring to include thin LVM
- Improved `systemctl` running schedule

#### Version 2.3.4:
- Improved Software RAID detection

#### Version 2.3.3:
- Implemented automatic ports detection if no ports specified in `luma.cfg`
- Improved CPU temperature collection method
- Improved disk metrics collection method
- Fixed an issue with Outgoing Pings validation

#### Version 2.3.2:
- Add support for HP Smart Array RAID controllers
- Fixed an issue where, in some cases, CPU temperature wasn't being read properly

#### Version 2.3.1:
- Improved `mdadm` RAID reading
- Improved auto network detection
- Improved install script to use `systemd`
- Fixed `lsblk` compatibility with older operating systems

#### Version 2.3.0:
- Added support for Outgoing Pings
- Added support for `zpool` pools IO read/write metrics
- Fallback service-status check via `service` command
- Improved NVMe disks info reading
- Improved disk IO read/write metrics
- Improved OS reading
- Branch-existence verification in the update script
- Fixed an issue where in some cases `sensors` would yield repeated errors (non-impacting)
- Fixed an issue where sometimes smart log would yield some errors (non-impacting)

#### Version 2.2.11:
- Fixed an issue where, in some cases, `zpool` metrics weren't being registered properly

#### Version 2.2.10:
- Fixed an issue where, in some cases, the CPU temperature collection would interrupt the metrics collection loop

#### Version 2.2.9:
- Fixed an issue where, in some cases, NVMe disk Serial and Model were not detected properly

#### Version 2.2.8:
- Fixed an issue with incorrect MegaRAID disk detection

#### Version 2.2.7:
- Fixed an issue related to `mdadm` introduced in `2.2.1`

#### Version 2.2.6:
- Switched back to the old DEBUG mode method, as it was more efficient
- Fixed an incompatibility with older kernels introduced in `2.2.5`
- Fixed cases where Software RAID isn't picked up properly

#### Version 2.2.5:
- `zpool` now records disk usage even for unmounted pools
- Fixed some rare cases where base64 would break on multiple lines

#### Version 2.2.4:
- `CPUModel` should now pick up more CPU model names
- Fixed an issue where `CPUCores` and `CPUThreads` would not handle multi-line data properly
- Changed how the agent handles killing of its own old/frozen processes, if any are found
- Increased DEBUG mode clearing time
- Improved DEBUG mode

#### Version 2.2.3:
- Fixed an issue where SMART test was not properly performed on NVMe disks
- Improved DEBUG mode

#### Version 2.2.2:
- Improved DEBUG mode
- Fixed an issue where agent would kill its own processes too eagerly, causing data sending issues

#### Version 2.2.1:
- Fixed an issue where, in some cases, the agent would query `mdadm` for devices not using `mdadm`
- Changed `LC_NUMERIC` locale

#### Version 2.2.0:
- Introducing DEBUG mode
- Minor fixes and tweaks

#### Version 2.1.0:
- Introducing ZFS pool health monitoring
- Added `CollectEveryXSeconds` to agent config

#### Version 2.0.12:
- Fixed an issue with the new service monitoring method introduced in version 2.0.11

#### Version 2.0.11:
- Improved service monitoring
- Fixed an issue where the metrics collection loop wouldn't break properly

#### Version 2.0.10: 
- Fixed `division by zero` error for servers without swap
- Replace `ifconfig` with `ip` command for IP address extraction (thanks to @Ry3nlNaToR)

#### Version 2.0.9: 
- Fixed a bug where in some cases RAID would not be detected properly

#### Version 2.0.8: 
- Added support for Custom Variables:
- Fixed `needs-restarting` for CloudLinux

#### Version 2.0.7:  
- Improved CPU temperature reading by adding support for two third-party software `lm-sensors` and `ipmitool`

#### Version 2.0.6:
- Improved the `servicestatus` function

#### Version 2.0.5:
- Improved the `servicestatus` function

#### Version 2.0.4:
- Fixed a bug where data was not properly being formatted in some cases when monitoring running processes

#### Version 2.0.3:
- Initial v2 release

#### Version 1.x:

