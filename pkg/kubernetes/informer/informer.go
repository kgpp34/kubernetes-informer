package informer

type Informer interface {
	Start(stopCh <-chan struct{})
	HasSynced() bool
}
