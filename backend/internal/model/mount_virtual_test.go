package model

import "testing"

func TestJoinMountVirtualPath(t *testing.T) {
	cases := []struct {
		name         string
		id           int64
		internalPath string
		want         string
	}{
		{"空路径为挂载根", 12, "", "webdav://12"},
		{"根斜杠", 12, "/", "webdav://12"},
		{"不带前导斜杠", 12, "移动云盘", "webdav://12/移动云盘"},
		{"带前导斜杠", 12, "/移动云盘", "webdav://12/移动云盘"},
		{"多级路径", 12, "移动云盘/电视剧/第一季", "webdav://12/移动云盘/电视剧/第一季"},
		{"多余斜杠", 7, "//a//b/", "webdav://7/a//b"},
		{"首尾空白", 3, "  电影  ", "webdav://3/电影"},
	}

	for _, tc := range cases {
		if got := JoinMountVirtualPath(tc.id, tc.internalPath); got != tc.want {
			t.Errorf("%s: JoinMountVirtualPath(%d, %q) = %q, want %q", tc.name, tc.id, tc.internalPath, got, tc.want)
		}
	}
}

func TestMountParentVirtualPath(t *testing.T) {
	cases := []struct {
		name         string
		internalPath string
		want         string
	}{
		{"挂载根没有上一级", "", ""},
		{"根斜杠没有上一级", "/", ""},
		{"一级目录回到挂载根", "移动云盘", ""},
		{"一级目录带斜杠回到挂载根", "/移动云盘", ""},
		{"二级目录回到一级", "移动云盘/电视剧", "webdav://12/移动云盘"},
		{"三级目录回到二级", "移动云盘/电视剧/第一季", "webdav://12/移动云盘/电视剧"},
		{"尾斜杠归一化", "移动云盘/电视剧/", "webdav://12/移动云盘"},
	}

	for _, tc := range cases {
		if got := MountParentVirtualPath(12, tc.internalPath); got != tc.want {
			t.Errorf("%s: MountParentVirtualPath(12, %q) = %q, want %q", tc.name, tc.internalPath, got, tc.want)
		}
	}
}
