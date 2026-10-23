#!/bin/bash
# Prints every 2 s: total CPU %, batnetd CPU % (of one core), batnetd RSS MB, available MB, temperature, throttled flags.
pid=$(systemctl show -p MainPID --value batnet-hub)
hz=$(getconf CLK_TCK)
read -r _ u n s i w q sq st _ < /proc/stat; t0=$((u+n+s+i+w+q+sq+st)); i0=$((i+w))
p0=$(awk '{print $14+$15}' /proc/$pid/stat); s0=$(date +%s.%N)
while sleep 2; do
  read -r _ u n s i w q sq st _ < /proc/stat; t1=$((u+n+s+i+w+q+sq+st)); i1=$((i+w))
  p1=$(awk '{print $14+$15}' /proc/$pid/stat); s1=$(date +%s.%N)
  rss=$(awk '/VmRSS/{print int($2/1024)}' /proc/$pid/status)
  av=$(awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo)
  temp=$(awk '{printf "%.1f", $1/1000}' /sys/class/thermal/thermal_zone0/temp)
  thr=$(vcgencmd get_throttled 2>/dev/null | cut -d= -f2)
  awk -v dt=$((t1-t0)) -v di=$((i1-i0)) -v dp=$((p1-p0)) -v hz=$hz -v s0=$s0 -v s1=$s1 -v rss=$rss -v av=$av -v temp=$temp -v thr=$thr \
    'BEGIN{printf "%s cpu=%.0f%% batnetd=%.0f%% rss=%dMB avail=%dMB temp=%sC thr=%s\n", strftime("%H:%M:%S"), 100*(dt-di)/dt, 100*dp/hz/(s1-s0), rss, av, temp, thr}'
  t0=$t1; i0=$i1; p0=$p1; s0=$s1
done
