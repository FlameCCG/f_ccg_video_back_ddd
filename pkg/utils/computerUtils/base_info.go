// Package computerUtils 采集主机 CPU、内存与磁盘使用信息。
//
// base_info.go 通过 gopsutil 查询系统资源供管理展示。
// 采集结果是某次观测，尤其 CPU 采样可能等待统计窗口，不应在高频请求中忽略采样成本。
package computerUtils

import (
	"errors"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

// GetCPUUsage 等待 interval 前后采样总 CPU 时间，计算 0～100 的使用百分比；采样失败返回 0。
func GetCPUUsage(interval time.Duration) (float64, error) {
	t1, err := cpu.Times(false)
	if err != nil || len(t1) == 0 {
		if err == nil {
			err = errors.New("no CPU samples")
		}

		return 0, fmt.Errorf("first CPU sample: %w", err)
	}

	time.Sleep(interval)

	t2, err := cpu.Times(false)
	if err != nil || len(t2) == 0 {
		if err == nil {
			err = errors.New("no CPU samples")
		}

		return 0, fmt.Errorf("second CPU sample: %w", err)
	}
	// 取第一个元素（false 表示汇总所有核心）
	cpu1 := t1[0]
	cpu2 := t2[0]
	idle1 := cpu1.Idle + cpu1.Iowait
	idle2 := cpu2.Idle + cpu2.Iowait
	nonIdle1 := cpu1.User + cpu1.Nice + cpu1.System + cpu1.Irq + cpu1.Softirq + cpu1.Steal
	nonIdle2 := cpu2.User + cpu2.Nice + cpu2.System + cpu2.Irq + cpu2.Softirq + cpu2.Steal
	total1 := idle1 + nonIdle1
	total2 := idle2 + nonIdle2
	totalDelta := total2 - total1
	idleDelta := idle2 - idle1

	if totalDelta == 0 {
		return 0, nil
	}

	cpuPercent := (1.0 - idleDelta/totalDelta) * 100.0

	return cpuPercent, nil
}

// GetMemoryUsage 返回当前物理内存已用百分比，采集失败记录日志并返回 0。
func GetMemoryUsage() (float64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	return vmStat.UsedPercent, nil
}

// GetDiskUsage 按可读取挂载点汇总已用/总容量计算百分比；无法读取的分区跳过。
func GetDiskUsage() (float64, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return 0, err
	}

	var (
		partialErr error
		totalUsed  uint64
		totalSize  uint64
	)

	for _, p := range partitions {
		usageStat, err := disk.Usage(p.Mountpoint)
		if err != nil {
			partialErr = errors.Join(partialErr, err)
			continue
		}

		if usageStat.Total > 0 {
			totalUsed += usageStat.Used
			totalSize += usageStat.Total
		}
	}

	if totalSize == 0 {
		return 0, partialErr
	}

	usedPercent := (float64(totalUsed) / float64(totalSize)) * 100

	return usedPercent, partialErr
}
