package search

import "context"

// 本文件是搜索应用层单测用的手写 stub：不走 wire、不依赖真实 ES/Redis/MySQL。

type fakeKeywordSearch struct {
	available bool
	multi     CombinedHits
	multiErr  error
	videos    VideoHits
	videoErr  error
	users     UserHits
	userErr   error
	userTotal int64
	countErr  error
	countCall int
}

func (f *fakeKeywordSearch) Available() bool { return f.available }

func (f *fakeKeywordSearch) MultiSearch(_ context.Context, _ SearchOptions, _ int) (CombinedHits, error) {
	return f.multi, f.multiErr
}

func (f *fakeKeywordSearch) SearchVideos(_ context.Context, _ SearchOptions, _ int) (VideoHits, error) {
	return f.videos, f.videoErr
}

func (f *fakeKeywordSearch) SearchUsers(_ context.Context, _ SearchOptions, _ int) (UserHits, error) {
	return f.users, f.userErr
}

func (f *fakeKeywordSearch) CountUsers(_ context.Context, _ string) (int64, error) {
	f.countCall++

	return f.userTotal, f.countErr
}

type fakeSearchFallback struct {
	available  bool
	videos     []SearchVideoHit
	videoTotal int64
	users      []SearchUserHit
	userTotal  int64
	countUsers int64
	meta       []VideoSearchMeta
	err        error
	metaCalls  [][]uint
}

func (f *fakeSearchFallback) Available() bool { return f.available }

func (f *fakeSearchFallback) SearchVideos(_ context.Context, _ SearchOptions) ([]SearchVideoHit, int64, error) {
	return f.videos, f.videoTotal, f.err
}

func (f *fakeSearchFallback) SearchUsers(_ context.Context, _ SearchOptions) ([]SearchUserHit, int64, error) {
	return f.users, f.userTotal, f.err
}

func (f *fakeSearchFallback) CountUsers(_ context.Context, _ string) (int64, error) {
	return f.countUsers, f.err
}

func (f *fakeSearchFallback) VideoMetaByIDs(_ context.Context, ids []uint) ([]VideoSearchMeta, error) {
	f.metaCalls = append(f.metaCalls, ids)

	return f.meta, nil
}

type fakeSuggestSearch struct {
	available bool
	items     map[string][]SuggestionItem
	err       error
}

func (f *fakeSuggestSearch) Available() bool { return f.available }

func (f *fakeSuggestSearch) FieldSuggest(_ context.Context, docType, _ string, _ int) ([]SuggestionItem, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.items[docType], nil
}

type fakeSuggestFallback struct {
	available bool
	titles    []string
	usernames []string
	tags      []string
	err       error
	sizes     map[string]int
}

func (f *fakeSuggestFallback) Available() bool { return f.available }

func (f *fakeSuggestFallback) record(kind string, size int) {
	if f.sizes == nil {
		f.sizes = make(map[string]int)
	}

	f.sizes[kind] = size
}

func (f *fakeSuggestFallback) VideoTitles(_ context.Context, _ string, size int) ([]string, error) {
	f.record("titles", size)

	return f.titles, f.err
}

func (f *fakeSuggestFallback) Usernames(_ context.Context, _ string, size int) ([]string, error) {
	f.record("usernames", size)

	return f.usernames, f.err
}

func (f *fakeSuggestFallback) TagNames(_ context.Context, _ string, size int) ([]string, error) {
	f.record("tags", size)

	return f.tags, f.err
}

type fakeSuggestionCache struct {
	entries map[string][]SuggestionItem
	stores  int
}

func newFakeSuggestionCache() *fakeSuggestionCache {
	return &fakeSuggestionCache{entries: make(map[string][]SuggestionItem)}
}

func (f *fakeSuggestionCache) Load(key string) ([]SuggestionItem, bool) {
	items, ok := f.entries[key]

	return items, ok
}

func (f *fakeSuggestionCache) Store(key string, items []SuggestionItem) {
	f.stores++
	f.entries[key] = items
}

type fakeHotKeywordStore struct {
	available   bool
	total       int64
	entries     []HotKeywordEntry
	cardErr     error
	rangeErr    error
	incremented []string
	lastRange   [2]int64
}

func (f *fakeHotKeywordStore) Available() bool { return f.available }

func (f *fakeHotKeywordStore) Card(_ context.Context) (int64, error) {
	return f.total, f.cardErr
}

func (f *fakeHotKeywordStore) RevRangeWithScores(_ context.Context, start, stop int64) ([]HotKeywordEntry, error) {
	f.lastRange = [2]int64{start, stop}

	return f.entries, f.rangeErr
}

func (f *fakeHotKeywordStore) Incr(_ context.Context, keyword string) {
	f.incremented = append(f.incremented, keyword)
}

type fakeHotKeywordFallback struct {
	available  bool
	rows       []HotVideoTitle
	err        error
	lastSample int
}

func (f *fakeHotKeywordFallback) Available() bool { return f.available }

func (f *fakeHotKeywordFallback) HotVideoTitles(_ context.Context, sampleSize int) ([]HotVideoTitle, error) {
	f.lastSample = sampleSize

	return f.rows, f.err
}

type fakeMentionRelations struct {
	users []MentionUser
	err   error
}

func (f *fakeMentionRelations) FollowingWithFriendFlag(_ context.Context, _ uint) ([]MentionUser, error) {
	return f.users, f.err
}

type fakeMentionSearch struct {
	available bool
	ids       []uint
	err       error
}

func (f *fakeMentionSearch) Available() bool { return f.available }

func (f *fakeMentionSearch) MatchUserIDs(_ context.Context, _ []uint, _ string) ([]uint, error) {
	return f.ids, f.err
}

type fakeUserDocumentQuery struct {
	docs      []UserSearchDocument
	err       error
	lastIDs   []uint
	callCount int
}

func (f *fakeUserDocumentQuery) LoadDocuments(_ context.Context, userIDs []uint) ([]UserSearchDocument, error) {
	f.callCount++
	f.lastIDs = userIDs

	return f.docs, f.err
}

type fakeUserDocumentIndexer struct {
	available bool
	err       error
	batches   [][]UserSearchDocument
}

func (f *fakeUserDocumentIndexer) Available() bool { return f.available }

func (f *fakeUserDocumentIndexer) BulkIndexUsers(_ context.Context, docs []UserSearchDocument) error {
	f.batches = append(f.batches, docs)

	return f.err
}
