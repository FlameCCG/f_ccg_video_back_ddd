// Package computerUtils provides package functionality.
package computerUtils

import (
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
)

// GetCpuUsage 导出符号.
func GetCpuUsage(interval time.Duration) float64 {
	t1, err := cpu.Times(false)
	if err != nil || len(t1) == 0 {
		logrus.Warnf("获取 CPU 时间失败（第一次采样）：%v", err)
		return 0
	}

	time.Sleep(interval)

	t2, err := cpu.Times(false)
	if err != nil || len(t2) == 0 {
		logrus.Warnf("获取 CPU 时间失败（第二次采样）：%v", err)
		return 0
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
		return 0
	}

	cpuPercent := (1.0 - idleDelta/totalDelta) * 100.0

	return cpuPercent
}

// GetMemoryUsage 导出符号.
func GetMemoryUsage() float64 {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		logrus.Warnf("获取内存使用情况失败：%v", err)
		return 0
	}

	return vmStat.UsedPercent
}

// GetDiskUsage 导出符号.
func GetDiskUsage() float64 {
	partitions, err := disk.Partitions(true)
	if err != nil {
		logrus.Warnf("获取磁盘分区失败：%v", err)
		return 0
	}

	var (
		totalUsed uint64
		totalSize uint64
	)

	for _, p := range partitions {
		usageStat, err := disk.Usage(p.Mountpoint)
		if err != nil {
			logrus.Warnf("获取 %s 磁盘使用情况失败：%v", p.Mountpoint, err)
			continue
		}

		if usageStat.Total > 0 {
			totalUsed += usageStat.Used
			totalSize += usageStat.Total
		}
	}

	if totalSize == 0 {
		return 0
	}

	usedPercent := (float64(totalUsed) / float64(totalSize)) * 100

	return usedPercent
}
