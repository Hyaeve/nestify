package model

import "testing"

// 路径判定必须卡在 id 边界上：webdav://1 不能命中 webdav://12，
// 否则删除 1 号挂载会误报成 12 号挂载被引用。
func TestReferencesMount(t *testing.T) {
	cases := []struct {
		name string
		path string
		id   int64
		want bool
	}{
		{"挂载根", "webdav://12", 12, true},
		{"挂载根带尾斜杠", "webdav://12/", 12, true},
		{"子层级", "webdav://12/移动云盘/电视剧", 12, true},
		{"首尾空白", "  webdav://12/剧集  ", 12, true},
		{"id 是另一个编号的前缀", "webdav://120/剧集", 12, false},
		{"相邻编号不算命中", "webdav://3/剧集", 12, false},
		{"本地路径", "/data/media/剧集", 12, false},
		{"空路径", "", 12, false},
		{"无编号前缀", "webdav:///剧集", 12, false},
		{"非法 id", "webdav://12/剧集", 0, false},
	}

	for _, c := range cases {
		if got := ReferencesMount(c.path, c.id); got != c.want {
			t.Errorf("%s: ReferencesMount(%q, %d) = %t, want %t", c.name, c.path, c.id, got, c.want)
		}
	}
}

// 编号即主键、删除后不重排，所以「挂载根」与「挂载根 + 子路径」都必须算命中，
// 否则删挂载时漏报引用，用户只会在规则跑失败时才发现。
func TestReferencesMountCoversRootAndChildren(t *testing.T) {
	id := int64(7)
	if !ReferencesMount(BuildMountVirtualPath(id), id) {
		t.Fatal("挂载根应算命中")
	}
	if !ReferencesMount(JoinMountVirtualPath(id, "/移动云盘/电视剧"), id) {
		t.Fatal("挂载内子路径应算命中")
	}
	if ReferencesMount(JoinMountVirtualPath(id+1, "/移动云盘"), id) {
		t.Fatal("另一个挂载不应算命中")
	}
}
