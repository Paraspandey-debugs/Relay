package manager

type Interface interface {
	Add(req AddRequest) (string, error)
	Pause(id string) error
	Resume(id string) error
	Remove(id string, cleanupPartials bool) error
	ReorderQueue(ids []string) error
	FindDuplicate(url, destination string) (DownloadRecord, bool)
	Get(id string) (DownloadRecord, bool)
	List() []DownloadRecord
	Queue() []string
	Subscribe() <-chan Event
	Unsubscribe(ch <-chan Event)
	SetMaxConcurrent(n int)
	SetDefaultWorkers(n int)
	GetMaxConcurrent() int
	GetDefaultWorkers() int
}
