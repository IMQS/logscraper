// +build !windows

package logscraper

func RunAsService(handler func()) bool {
	return false
}
