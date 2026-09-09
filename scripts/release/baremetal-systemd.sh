#!/bin/bash
# Only frozen artifacts are deployed. No compiler, container or package manager.
set -euo pipefail
[[ $# == 4 ]] || { echo 'usage: baremetal-systemd.sh BUNDLE RUN_ID REVISION calibration|soak' >&2; exit 2; }
bundle=$1
run_id=$2
revision=$3
mode=$4
[[ $(id -u) == 0 ]] || { echo 'root is required only to create the bounded transient service' >&2; exit 1; }
[[ $bundle =~ ^/opt/rjs-qualification/[a-zA-Z0-9._-]+$ && $run_id =~ ^[a-z0-9][a-z0-9-]{1,48}$ && $revision =~ ^[a-f0-9]{40}$ ]] || exit 2
[[ $(realpath -e -- "$bundle") == "$bundle" && -d $bundle ]] || exit 2
[[ $mode == calibration || $mode == soak ]] || exit 2
unit="rjs-qual-$run_id"
state="/var/lib/$unit"
[[ ! -e $state && ! -L $state && ! -e /var/lib/private/$unit && ! -L /var/lib/private/$unit ]] || { echo 'state already exists; refusing reuse' >&2; exit 1; }
[[ $(systemctl show "$unit.service" -p LoadState --value) == not-found ]] || { echo 'unit already exists; refusing replacement' >&2; exit 1; }
[[ -z $(find "$bundle" -xdev \( ! -user root -o -perm /022 -o -type l \) -print -quit) ]] || { echo 'bundle must be immutable to non-root users and contain no links' >&2; exit 1; }
(cd "$bundle" && sha256sum --strict --check SHA256SUMS)
[[ $(systemd-detect-virt || true) == none ]] || { echo 'this launch profile requires a physical host' >&2; exit 1; }
[[ $(stat -fc %T /sys/fs/cgroup) == cgroup2fs ]] || exit 1
device=$(findmnt -n -o SOURCE -T /var/lib)
[[ -b $device ]] || { echo 'cannot bind IO limit to state filesystem device' >&2; exit 1; }
args=(-duration 24h -port-base 24240)
if [[ $mode == calibration ]]; then args=(-calibrate -duration 2m -port-base 24220); fi
# No enable, restart policy, account modification, installation or global configuration.
systemd-run --unit="$unit" --description="Isolated RJS bare-metal $mode ($run_id)" \
 --property=Type=exec --property=DynamicUser=yes --property="StateDirectory=$unit" --property=StateDirectoryMode=0700 \
 --property=ProtectSystem=strict --property=ProtectHome=yes --property=PrivateTmp=yes --property=PrivateDevices=yes \
 --property=NoNewPrivileges=yes --property=ProtectKernelTunables=yes --property=ProtectKernelModules=yes --property=ProtectControlGroups=yes \
 --property='RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6' --property=IPAddressDeny=any --property=IPAddressAllow=localhost \
 --property=CPUQuota=400% --property=CPUWeight=10 --property=MemoryHigh=3G --property=MemoryMax=4G --property=MemorySwapMax=0 \
 --property=IOWeight=10 --property="IOWriteBandwidthMax=$device 20M" --property="IOReadBandwidthMax=$device 40M" \
 --property=Nice=10 --property=IOSchedulingClass=idle --property=TasksMax=128 --property=LimitNOFILE=4096 --property=LimitFSIZE=256M \
 --property=RuntimeMaxSec=26h --property=TimeoutStopSec=30s --property=Restart=no --property=KillMode=control-group --property=OOMPolicy=stop \
 --property="StandardOutput=append:$state/supervisor.log" --property="StandardError=append:$state/supervisor.log" \
 --setenv=RJS_BAREMETAL_CONFINED=systemd-v1 --setenv=GOMAXPROCS=2 \
 "$bundle/bin/linux-amd64/baremetal-run" -bundle "$bundle" -output "$state" -source-revision "$revision" "${args[@]}"
systemctl show "$unit.service" -p Id -p MainPID -p DynamicUser -p User -p StateDirectory -p CPUQuotaPerSecUSec -p MemoryHigh -p MemoryMax -p MemorySwapMax -p IOWriteBandwidthMax -p IOReadBandwidthMax -p TasksMax -p RuntimeMaxUSec -p IPAddressDeny -p IPAddressAllow -p ProtectSystem -p KillMode
