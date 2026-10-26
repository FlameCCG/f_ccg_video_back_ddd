package media

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanTranscodeChunks(t *testing.T) {
	t.Parallel()

	t.Run("even split", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3600, 4, 180)
		require.Len(t, chunks, 20, "chunks = %d, want 20", len(chunks))

		for i, ch := range chunks {
			require.Equal(t, ch.Index, i, "chunk %d = %+v, want start=%d len=180", i, ch, i*180)
			require.Equal(t, ch.StartSec, i*180, "chunk %d = %+v, want start=%d len=180", i, ch, i*180)
			require.Equal(t, 180, ch.LenSec, "chunk %d = %+v, want start=%d len=180", i, ch, i*180)
		}
	})

	t.Run("plan is deterministic across calls", func(t *testing.T) {
		t.Parallel()

		a := PlanTranscodeChunks(3777, 4, 180)

		b := PlanTranscodeChunks(3777, 4, 180)
		require.False(t, len(a) == 0 || len(a) != len(b), "plan lengths differ: %d vs %d", len(a), len(b))

		for i := range a {
			require.Equal(t, a[i], b[i], "chunk %d differs: %+v vs %+v", i, a[i], b[i])
		}
	})

	t.Run("tiny tail merges into previous chunk", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3601, 4, 180)
		require.Len(t, chunks, 20, "chunks = %d, want 20 (tail merged)", len(chunks))

		last := chunks[len(chunks)-1]
		require.Equal(t, 181, last.LenSec, "merged last chunk len = %d, want 181", last.LenSec)
	})

	t.Run("normal tail keeps own chunk", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(3700, 4, 180)
		require.Len(t, chunks, 21, "chunks = %d, want 21", len(chunks))

		last := chunks[len(chunks)-1]
		require.Equal(t, 3600, last.StartSec, "last chunk = %+v, want start=3600 len=100", last)
		require.Equal(t, 100, last.LenSec, "last chunk = %+v, want start=3600 len=100", last)
	})

	t.Run("chunk length rounds up to seg duration grid", func(t *testing.T) {
		t.Parallel()

		chunks := PlanTranscodeChunks(1000, 4, 90)
		require.NotEmpty(t, chunks, "expected chunked plan")

		require.Equal(t, 0, chunks[0].LenSec%4, "chunk len %d not aligned to segDur grid", chunks[0].LenSec)
	})

	t.Run("short source not chunked", func(t *testing.T) {
		t.Parallel()

		{
			chunks := PlanTranscodeChunks(359, 4, 180)
			require.Nil(t, chunks, "short source must not be chunked, got %d chunks", len(chunks))
		}

		{
			chunks := PlanTranscodeChunks(0, 4, 180)
			require.Nil(t, chunks, "zero duration must not be chunked")
		}
	})

	t.Run("coverage is exact and gapless", func(t *testing.T) {
		t.Parallel()

		duration := 4567
		chunks := PlanTranscodeChunks(duration, 4, 180)
		next := 0
		total := 0

		for _, ch := range chunks {
			require.Equal(t, ch.StartSec, next, "gap before chunk %d: start=%d want=%d", ch.Index, ch.StartSec, next)

			next = ch.StartSec + ch.LenSec
			total += ch.LenSec
		}

		require.Equal(t, total, duration, "chunk coverage = %d, want %d", total, duration)
	})
}

// TestChunkPlanGates 覆盖运行时切块入口的三个关闭闸门
// （原 transcode_chunked_test.go 的 TestTranscodeChunkPlanGates：
// ephemeral/nil 断点与 chunkDurationSec<0，现由 checkpointPersistent 形参与策略字段表达）。
func TestChunkPlanGates(t *testing.T) {
	t.Parallel()

	policy := &TranscodePolicy{DashSegDuration: 4}

	{
		plan := policy.ChunkPlan(3600, true)
		require.NotEmpty(t, plan, "long source with persistent checkpoint must be chunked")
	}

	// 断点不可跨尝试复用（ephemeral / 无断点目录）：分段无意义。
	{
		plan := policy.ChunkPlan(3600, false)
		require.Nil(t, plan, "non-persistent checkpoint must not chunk")
	}

	// chunkDurationSec < 0 显式关闭分段。
	disabled := &TranscodePolicy{DashSegDuration: 4, ChunkDurationSec: -1}
	{
		plan := disabled.ChunkPlan(3600, true)
		require.Nil(t, plan, "negative chunkDurationSec must disable chunking")
	}

	// nil 策略走默认块长，仍可分段。
	var nilPolicy *TranscodePolicy
	{
		plan := nilPolicy.ChunkPlan(3600, true)
		require.NotEmpty(t, plan, "nil policy must fall back to default chunk length")
	}
}

func TestChunkArtifactDirNameSeparatesEncoderModes(t *testing.T) {
	t.Parallel()

	gpu := ChunkArtifactDirName(TranscodePipelineMP4, true, 3)

	cpu := ChunkArtifactDirName(TranscodePipelineMP4, false, 3)
	require.NotEqual(t, gpu, cpu, "gpu/cpu chunk namespaces must differ (mixing encoders in one concat is forbidden)")

	require.Contains(t, gpu, "gpu", "dir names must embed encoder mode: %q %q", gpu, cpu)
	require.Contains(t, cpu, "cpu", "dir names must embed encoder mode: %q %q", gpu, cpu)

	require.NotEqual(t, ChunkArtifactDirName(TranscodePipelineDASH, false, 3), cpu, "pipeline namespaces must differ")
}

// TestIsLastChunk 覆盖末块判定（原实现为内联的 chunk.Index == plan[len-1].Index 比较）。
func TestIsLastChunk(t *testing.T) {
	t.Parallel()

	plan := PlanTranscodeChunks(3600, 4, 180)
	require.GreaterOrEqual(t, len(plan), 2, "expected multi-chunk plan, got %d", len(plan))

	require.True(t, IsLastChunk(plan, plan[len(plan)-1]), "last chunk must be detected")

	require.False(t, IsLastChunk(plan, plan[0]), "first chunk must not be treated as last")

	require.False(t, IsLastChunk(nil, plan[0]), "empty plan has no last chunk")
}
