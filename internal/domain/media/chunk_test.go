package media

import (
	"strings"
	"testing"
)

func TestPlanTranscodeChunks(t *testing.T) {
	t.Parallel()

	t.Run("even split", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3600, 4, 180)
		if len(chunks) != 20 {
			t.Fatalf("chunks = %d, want 20", len(chunks))
		}

		for i, ch := range chunks {
			if ch.Index != i || ch.StartSec != i*180 || ch.LenSec != 180 {
				t.Fatalf("chunk %d = %+v, want start=%d len=180", i, ch, i*180)
			}
		}
	})

	t.Run("plan is deterministic across calls", func(t *testing.T) {
		t.Parallel()

		a := PlanTranscodeChunks(3777, 4, 180)

		b := PlanTranscodeChunks(3777, 4, 180)
		if len(a) == 0 || len(a) != len(b) {
			t.Fatalf("plan lengths differ: %d vs %d", len(a), len(b))
		}

		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("chunk %d differs: %+v vs %+v", i, a[i], b[i])
			}
		}
	})

	t.Run("tiny tail merges into previous chunk", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3601, 4, 180)
		if len(chunks) != 20 {
			t.Fatalf("chunks = %d, want 20 (tail merged)", len(chunks))
		}

		last := chunks[len(chunks)-1]
		if last.LenSec != 181 {
			t.Fatalf("merged last chunk len = %d, want 181", last.LenSec)
		}
	})

	t.Run("normal tail keeps own chunk", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3700, 4, 180)
		if len(chunks) != 21 {
			t.Fatalf("chunks = %d, want 21", len(chunks))
		}

		last := chunks[len(chunks)-1]
		if last.StartSec != 3600 || last.LenSec != 100 {
			t.Fatalf("last chunk = %+v, want start=3600 len=100", last)
		}
	})

	t.Run("chunk length rounds up to seg duration grid", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(1000, 4, 90)
		if len(chunks) == 0 {
			t.Fatal("expected chunked plan")
		}

		if chunks[0].LenSec%4 != 0 {
			t.Fatalf("chunk len %d not aligned to segDur grid", chunks[0].LenSec)
		}
	})

	t.Run("short source not chunked", func(t *testing.T) {
		t.Parallel()

		if chunks := PlanTranscodeChunks(359, 4, 180); chunks != nil {
			t.Fatalf("short source must not be chunked, got %d chunks", len(chunks))
		}

		if chunks := PlanTranscodeChunks(0, 4, 180); chunks != nil {
			t.Fatal("zero duration must not be chunked")
		}
	})

	t.Run("coverage is exact and gapless", func(t *testing.T) {
		t.Parallel()

		duration := 4567
		chunks := PlanTranscodeChunks(duration, 4, 180)
		next := 0
		total := 0

		for _, ch := range chunks {
			if ch.StartSec != next {
				t.Fatalf("gap before chunk %d: start=%d want=%d", ch.Index, ch.StartSec, next)
			}

			next = ch.StartSec + ch.LenSec
			total += ch.LenSec
		}

		if total != duration {
			t.Fatalf("chunk coverage = %d, want %d", total, duration)
		}
	})
}

// TestChunkPlanGates 覆盖运行时切块入口的三个关闭闸门
// （原 transcode_chunked_test.go 的 TestTranscodeChunkPlanGates：
// ephemeral/nil 断点与 chunkDurationSec<0，现由 checkpointPersistent 形参与策略字段表达）。
func TestChunkPlanGates(t *testing.T) {
	t.Parallel()

	policy := &TranscodePolicy{DashSegDuration: 4}

	if plan := policy.ChunkPlan(3600, true); len(plan) == 0 {
		t.Fatal("long source with persistent checkpoint must be chunked")
	}

	// 断点不可跨尝试复用（ephemeral / 无断点目录）：分段无意义。
	if plan := policy.ChunkPlan(3600, false); plan != nil {
		t.Fatal("non-persistent checkpoint must not chunk")
	}

	// chunkDurationSec < 0 显式关闭分段。
	disabled := &TranscodePolicy{DashSegDuration: 4, ChunkDurationSec: -1}
	if plan := disabled.ChunkPlan(3600, true); plan != nil {
		t.Fatal("negative chunkDurationSec must disable chunking")
	}

	// nil 策略走默认块长，仍可分段。
	var nilPolicy *TranscodePolicy
	if plan := nilPolicy.ChunkPlan(3600, true); len(plan) == 0 {
		t.Fatal("nil policy must fall back to default chunk length")
	}
}

func TestChunkArtifactDirNameSeparatesEncoderModes(t *testing.T) {
	t.Parallel()

	gpu := ChunkArtifactDirName(TranscodePipelineMP4, true, 3)

	cpu := ChunkArtifactDirName(TranscodePipelineMP4, false, 3)
	if gpu == cpu {
		t.Fatal("gpu/cpu chunk namespaces must differ (mixing encoders in one concat is forbidden)")
	}

	if !strings.Contains(gpu, "gpu") || !strings.Contains(cpu, "cpu") {
		t.Fatalf("dir names must embed encoder mode: %q %q", gpu, cpu)
	}

	if ChunkArtifactDirName(TranscodePipelineDASH, false, 3) == cpu {
		t.Fatal("pipeline namespaces must differ")
	}
}

// TestIsLastChunk 覆盖末块判定（原实现为内联的 chunk.Index == plan[len-1].Index 比较）。
func TestIsLastChunk(t *testing.T) {
	t.Parallel()

	plan := PlanTranscodeChunks(3600, 4, 180)
	if len(plan) < 2 {
		t.Fatalf("expected multi-chunk plan, got %d", len(plan))
	}

	if IsLastChunk(plan, plan[len(plan)-1]) != true {
		t.Fatal("last chunk must be detected")
	}

	if IsLastChunk(plan, plan[0]) {
		t.Fatal("first chunk must not be treated as last")
	}

	if IsLastChunk(nil, plan[0]) {
		t.Fatal("empty plan has no last chunk")
	}
}
