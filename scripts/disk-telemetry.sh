# Disk I/O telemetry for provisioning-tests.
#
# Usage:
#   source ./scripts/disk-telemetry.sh
#   disk_telemetry_start                  # call once, early, before the test run starts
#   ...
#   kill $DISK_SAMPLER_PID 2>/dev/null     # in cleanup(), alongside other background samplers
#   disk_telemetry_report                 # in cleanup(), prints the summary/correlation/dump

disk_dev_snapshot()
{
    awk -v d="$DISK_DEV" '$3==d{print $4,$6,$7,$8,$10,$11,$12,$13}' /proc/diskstats
}

# Sets up DISK_DEV/DISK_START/DISK_TS_CSV/EBS_TS_CSV, resolves and installs nvme-cli when the
# underlying device is NVMe, and starts the background sampler (DISK_SAMPLER_PID).
disk_telemetry_start()
{
    # Snapshot /proc/diskstats now; disk_telemetry_report() diffs against this to report avg
    # I/O latency and %util for the whole run (time spent on I/Os / I/Os completed, and /
    # elapsed time).
    # Target the device backing etcd's data dir, not the repo bind mount. k3s etcd writes to
    # /var/lib/rancher (a separate VOLUME in Dockerfile.runtime), which may sit on a different
    # device than the current working directory - measuring `.` can therefore miss all the etcd
    # write pressure entirely. Fall back to `.` if /var/lib/rancher isn't resolvable yet.
    DISK_TARGET=/var/lib/rancher
    [ -d "$DISK_TARGET" ] || DISK_TARGET=.
    DISK_DEV=$(basename "$(df --output=source "$DISK_TARGET" | tail -1)")
    echo "disk telemetry: sampling device '$DISK_DEV' (backing $DISK_TARGET)"
    DISK_START=$(disk_dev_snapshot)
    DISK_TS_CSV=/tmp/disk-timeseries.csv
    EBS_TS_CSV=/tmp/nvme-ebs-timeseries.csv
    CORRELATION_CSV=/tmp/etcd-disk-correlation.csv
    echo "epoch,elapsed_s,queue_depth,r_iops,w_iops,r_kBps,w_kBps,r_await_ms,w_await_ms,util_pct" > "$DISK_TS_CSV"

    # On Nitro EC2 instances, EBS volumes attached as NVMe expose a vendor log page (0xD0) with
    # cumulative microseconds of IOPS/throughput throttling, split into volume-level (ebs_*) and
    # instance-level EBS-bandwidth-cap (ec2_*) counters. That split is exactly what's needed to
    # tell "this EBS volume is undersized" apart from "this instance type's EBS bandwidth is
    # undersized" - install nvme-cli on demand and best-effort parse it below.
    NVME_DEV=""
    NVME_CLI_OK=0
    if [[ "$DISK_DEV" == nvme* ]]; then
      NVME_DEV="/dev/$(echo "$DISK_DEV" | sed -E 's/(nvme[0-9]+n[0-9]+).*/\1/')"
      if command -v nvme >/dev/null 2>&1 || zypper -n install nvme-cli >/tmp/nvme-cli-install.log 2>&1; then
        NVME_CLI_OK=1
      else
        echo "WARNING: nvme-cli install failed, skipping EBS throttle telemetry"
      fi
    fi

    # Background sampler: every 5s, diff /proc/diskstats against the previous sample
    # (iostat-style) to get a timestamped time-series instead of a single whole-run average, so
    # exact spikes can be lined up against etcd log timestamps in disk_telemetry_report() below.
    # Also samples the NVMe EBS log page 0xD0 on the same cadence when available.
    (
        set +e
        read -r p_rc p_rsec p_rms p_wc p_wsec p_wms _ p_busy < <(disk_dev_snapshot)
        p_epoch=$(date +%s)
        EBS_MAGIC_CHECKED=0
        EBS_MAGIC_OK=0

        while true; do
            sleep 5

            read -r rc rsec rms wc wsec wms q busy < <(disk_dev_snapshot)
            epoch=$(date +%s)
            dt=$((epoch - p_epoch))
            [ "$dt" -le 0 ] && dt=5

            awk -v epoch="$epoch" -v elapsed="$SECONDS" -v q="$q" -v dt="$dt" \
                -v drc="$((rc - p_rc))" -v drsec="$((rsec - p_rsec))" -v drms="$((rms - p_rms))" \
                -v dwc="$((wc - p_wc))" -v dwsec="$((wsec - p_wsec))" -v dwms="$((wms - p_wms))" \
                -v dbusy="$((busy - p_busy))" '
                BEGIN {
                    r_iops = drc / dt; w_iops = dwc / dt
                    r_kBps = (drsec * 512 / 1024) / dt; w_kBps = (dwsec * 512 / 1024) / dt
                    r_await = drc > 0 ? drms / drc : 0; w_await = dwc > 0 ? dwms / dwc : 0
                    util = dbusy / 10 / dt
                    printf "%d,%d,%d,%.1f,%.1f,%.1f,%.1f,%.2f,%.2f,%.1f\n", epoch, elapsed, q, r_iops, w_iops, r_kBps, w_kBps, r_await, w_await, util
                }' >> "$DISK_TS_CSV"

            p_rc=$rc; p_rsec=$rsec; p_rms=$rms; p_wc=$wc; p_wsec=$wsec; p_wms=$wms; p_busy=$busy; p_epoch=$epoch

            if [ -n "$NVME_DEV" ] && [ "$NVME_CLI_OK" = 1 ] && nvme get-log "$NVME_DEV" --log-id=0xD0 --log-len=512 -b > /tmp/nvme-ebs-log.bin 2>/dev/null; then
                if [ "$EBS_MAGIC_CHECKED" = 0 ]; then
                    EBS_MAGIC_CHECKED=1
                    magic=$(od -An -tu8 -j0 -N8 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    # 0x3C23B510, the documented magic for the Amazon EBS NVMe log page struct.
                    if [ "$magic" = "1008973072" ]; then
                        EBS_MAGIC_OK=1
                        echo "epoch,elapsed_s,ebs_iops_exceeded_us,ebs_throughput_exceeded_us,ec2_iops_exceeded_us,ec2_throughput_exceeded_us,queue_length" > "$EBS_TS_CSV"
                    else
                        od -An -tx1 /tmp/nvme-ebs-log.bin > /tmp/nvme-ebs-raw.hex
                        echo "WARNING: NVMe EBS log page 0xD0 magic mismatch (got $magic, expected 1008973072) - struct layout differs on this host; raw hex dumped to /tmp/nvme-ebs-raw.hex; disabling further EBS throttle sampling"
                    fi
                fi
                if [ "$EBS_MAGIC_OK" = 1 ]; then
                    ebs_iops=$(od -An -tu8 -j64 -N8 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    ebs_tp=$(od -An -tu8 -j72 -N8 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    ec2_iops=$(od -An -tu8 -j80 -N8 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    ec2_tp=$(od -An -tu8 -j88 -N8 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    qlen=$(od -An -tu4 -j96 -N4 /tmp/nvme-ebs-log.bin | tr -d ' ')
                    echo "$epoch,$SECONDS,$ebs_iops,$ebs_tp,$ec2_iops,$ec2_tp,$qlen" >> "$EBS_TS_CSV"
                fi
            fi
        done
    ) &
    DISK_SAMPLER_PID=$!
}

# Grep etcd/k3s logs for known slow-write markers and print the nearest disk/EBS sample for
# each, so a queue/throttle spike can be tied directly to the etcd stall it (maybe) caused.
# Writes the same correlation to $CORRELATION_CSV (one row per matched log line) so it can be
# opened in a spreadsheet or diffed across runs, instead of only being eyeballed in stdout.
correlate_etcd_disk_stalls()
{
    local logfile line ts epoch disk_row ebs_row cpu_row csv_msg
    echo "epoch,timestamp,logfile,message,queue_depth,r_iops,w_iops,r_kBps,w_kBps,r_await_ms,w_await_ms,util_pct,ebs_iops_exceeded_us,ebs_throughput_exceeded_us,ec2_iops_exceeded_us,ec2_throughput_exceeded_us,ebs_queue_length,max_core_busy_pct,avg_core_busy_pct,agg_iowait_pct,agg_steal_pct,runq" > "$CORRELATION_CSV"

    for logfile in /tmp/rancher.log build/testdata/k3s.log; do
        [ -f "$logfile" ] || continue
        grep -E 'slow fdatasync|apply request took too long|wal: sync duration|leader failed to send out heartbeat on time' "$logfile" 2>/dev/null | while IFS= read -r line; do
            ts=$(echo "$line" | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z?' | head -1)
            epoch=""
            [ -n "$ts" ] && epoch=$(date -d "$ts" +%s 2>/dev/null)
            echo "$line"

            disk_row=""
            ebs_row=""
            cpu_row=""
            if [ -n "$epoch" ]; then
                # Field indices below pull just the metric columns (skip epoch/elapsed_s) from
                # the closest-by-epoch row of each time-series CSV, so this row can be appended
                # directly under the correlation header without re-deriving column order.
                disk_row=$(awk -F, -v t="$epoch" 'NR>1{d=$1-t; if(d<0)d=-d; if(!found || d<best){best=d; found=1; row=$3","$4","$5","$6","$7","$8","$9","$10}} END{if(found) print row}' "$DISK_TS_CSV")
                [ -n "$disk_row" ] && echo "  -> nearest disk sample (queue_depth,r_iops,w_iops,r_kBps,w_kBps,r_await_ms,w_await_ms,util_pct): $disk_row"
                if [ -s "$EBS_TS_CSV" ]; then
                    ebs_row=$(awk -F, -v t="$epoch" 'NR>1{d=$1-t; if(d<0)d=-d; if(!found || d<best){best=d; found=1; row=$3","$4","$5","$6","$7}} END{if(found) print row}' "$EBS_TS_CSV")
                    [ -n "$ebs_row" ] && echo "  -> nearest EBS sample (ebs_iops_exceeded_us,ebs_throughput_exceeded_us,ec2_iops_exceeded_us,ec2_throughput_exceeded_us,queue_length): $ebs_row"
                fi
                # The per-core CPU CSV is written by the sampler in scripts/provisioning-tests; pull
                # the nearest sample so an etcd stall can be attributed to a single-core CPU spike
                # (max_core_busy near 100 while util/queue are low) vs a disk stall vs steal.
                if [ -s /tmp/cpu-percore.csv ]; then
                    cpu_row=$(awk -F, -v t="$epoch" 'NR>1{d=$1-t; if(d<0)d=-d; if(!found || d<best){best=d; found=1; row=$4","$5","$6","$7","$8}} END{if(found) print row}' /tmp/cpu-percore.csv)
                    [ -n "$cpu_row" ] && echo "  -> nearest CPU sample (max_core_busy,avg_core_busy,iowait,steal,runq): $cpu_row"
                fi
            fi

            csv_msg=$(printf '%s' "$line" | sed 's/"/""/g')
            printf '%s,%s,%s,"%s",%s,%s,%s\n' "$epoch" "$ts" "$logfile" "$csv_msg" "${disk_row:-,,,,,,,}" "${ebs_row:-,,,,}" "${cpu_row:-,,,,}" >> "$CORRELATION_CSV"
        done
    done
}

# Dump the last ~60s of the disk/EBS time-series. Called from the crash handler in
# build_and_run_rancher when the embedded k3s/Rancher process dies mid-run, so the disk state in
# the seconds leading up to the death is visible right there in the log - the run-wide summary in
# disk_telemetry_report() only prints once, in cleanup(), and averages can't explain a crash.
# For the EBS counters, print several consecutive rows (not just the last) so the *delta* across
# them - the throttling that actually accrued near the death - is visible, since the raw values
# are cumulative since boot.
disk_telemetry_death_snapshot()
{
    echo "=== last ~60s disk time-series before death ==="
    if [ -s "$DISK_TS_CSV" ]; then
        head -1 "$DISK_TS_CSV"
        tail -12 "$DISK_TS_CSV"
    else
        echo "(no disk samples yet)"
    fi
    if [ -s "$EBS_TS_CSV" ] && [ "$(wc -l < "$EBS_TS_CSV")" -gt 1 ]; then
        echo "--- EBS throttle counters (cumulative us since boot; watch the delta across rows) ---"
        head -1 "$EBS_TS_CSV"
        tail -12 "$EBS_TS_CSV"
    fi
}

# Prints the whole-run disk/EBS summary, the etcd correlation, and a gzip+base64 dump of the raw
# time-series (so it survives in the job log even on passing runs), then removes the temp files.
# Call from cleanup(), after DISK_SAMPLER_PID has been killed.
disk_telemetry_report()
{
    read -r s_rc s_rsec s_rms s_wc s_wsec s_wms _ s_busy <<< "$DISK_START"
    read -r e_rc e_rsec e_rms e_wc e_wsec e_wms _ e_busy <<< "$(disk_dev_snapshot)"
    awk -v secs="$SECONDS" -v rc="$((e_rc - s_rc))" -v rt="$((e_rms - s_rms))" -v wc="$((e_wc - s_wc))" -v wt="$((e_wms - s_wms))" -v busy="$((e_busy - s_busy))" \
        'BEGIN{ if(rc+wc>0) printf "Disk Avg Latency: %.2fms (reads: %.2fms, writes: %.2fms) | Busy: %.1f%%\n", (rt+wt)/(rc+wc), (rc>0)?(rt/rc):0, (wc>0)?(wt/wc):0, (secs>0)?(busy/10/secs):0 }'
    awk -F, 'NR>1{q=$3; s+=q; n++; m=n==1||q<m?q:m; x=q>x?q:x} END{if(n) printf "Disk Queue Depth Min: %d | Max: %d | Avg: %.1f\n", m, x, s/n}' "$DISK_TS_CSV"
    if [ -s "$EBS_TS_CSV" ] && [ "$(wc -l < "$EBS_TS_CSV")" -gt 1 ]; then
        # Counters are cumulative microseconds since boot, so diff the first sample against the
        # last to get the throttling that accrued *during this run* - the raw tail also includes
        # unrelated bursts like the docker image load that ran before the test started.
        awk -F, 'NR==2{fi=$3;ft=$4;ci=$5;ct=$6} END{printf "EBS Throttle DURING RUN (delta) -- Vol IOPS: %.1fms | Vol Throughput: %.1fms || EC2 bandwidth IOPS: %.1fms | Throughput: %.1fms | Final Queue Length: %s\n", ($3-fi)/1000, ($4-ft)/1000, ($5-ci)/1000, ($6-ct)/1000, $7}' "$EBS_TS_CSV"
    fi

    echo ""
    echo "=== etcd slow-write / disk correlation ==="
    correlate_etcd_disk_stalls

    echo ""
    echo "=== Disk telemetry dump ==="
    echo -e "-----DISK-TELEMETRY-DUMP-START-----"
    {
        echo "## disk-timeseries.csv"
        cat "$DISK_TS_CSV" 2>/dev/null
        if [ -s "$EBS_TS_CSV" ]; then
            echo "## nvme-ebs-timeseries.csv"
            cat "$EBS_TS_CSV"
        fi
        if [ -s /tmp/cpu-percore.csv ]; then
            echo "## cpu-percore.csv"
            cat /tmp/cpu-percore.csv
        fi
        if [ -s "$CORRELATION_CSV" ]; then
            echo "## etcd-disk-correlation.csv"
            cat "$CORRELATION_CSV"
        fi
        if [ -s /tmp/nvme-ebs-raw.hex ]; then
            echo "## nvme-ebs-raw.hex (magic mismatch, unparsed)"
            cat /tmp/nvme-ebs-raw.hex
        fi
    } | gzip | base64 -w 0
    echo -e "\n-----DISK-TELEMETRY-DUMP-END-----"

    rm -f "$DISK_TS_CSV" "$EBS_TS_CSV" "$CORRELATION_CSV" /tmp/nvme-ebs-log.bin /tmp/nvme-ebs-raw.hex
}
