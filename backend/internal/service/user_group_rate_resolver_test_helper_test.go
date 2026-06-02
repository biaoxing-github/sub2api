//go:build unit

package service

func resetUserGroupRateCacheVersionForTest() {
	userGroupRateCacheVersion.Store(0)
}
