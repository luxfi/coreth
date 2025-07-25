// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package downloader

import (
	"context"
	"sync"
	"time"

	"github.com/luxfi/geth"
	"github.com/luxfi/geth/core"
	"github.com/luxfi/geth/event"
	"github.com/luxfi/geth/rpc"
)

// DownloaderAPI provides an API which gives information about the current
// synchronisation status. It offers only methods that operates on data that
// can be available to anyone without security risks.
type DownloaderAPI struct {
	d                         *Downloader
	chain                     *core.BlockChain
	mux                       *event.TypeMux
	installSyncSubscription   chan chan interface{}
	uninstallSyncSubscription chan *uninstallSyncSubscriptionRequest
}

// NewDownloaderAPI creates a new DownloaderAPI. The API has an internal event loop that
// listens for events from the downloader through the global event mux. In case it receives one of
// these events it broadcasts it to all syncing subscriptions that are installed through the
// installSyncSubscription channel.
func NewDownloaderAPI(d *Downloader, chain *core.BlockChain, m *event.TypeMux) *DownloaderAPI {
	api := &DownloaderAPI{
		d:                         d,
		chain:                     chain,
		mux:                       m,
		installSyncSubscription:   make(chan chan interface{}),
		uninstallSyncSubscription: make(chan *uninstallSyncSubscriptionRequest),
	}
	go api.eventLoop()
	return api
}

// eventLoop runs a loop until the event mux closes. It will install and uninstall
// new sync subscriptions and broadcasts sync status updates to the installed sync
// subscriptions.
//
// The sync status pushed to subscriptions can be a stream like:
// >>> {Syncing: true, Progress: {...}}
// >>> {false}
//
// If the node is already synced up, then only a single event subscribers will
// receive is {false}.
func (api *DownloaderAPI) eventLoop() {
	var (
		sub               = api.mux.Subscribe(StartEvent{})
		syncSubscriptions = make(map[chan interface{}]struct{})
		checkInterval     = time.Second * 60
		checkTimer        = time.NewTimer(checkInterval)

		// status flags
		started bool
		done    bool

		getProgress = func() ethereum.SyncProgress {
			prog := api.d.Progress()
			if txProg, err := api.chain.TxIndexProgress(); err == nil {
				prog.TxIndexFinishedBlocks = txProg.Indexed
				prog.TxIndexRemainingBlocks = txProg.Remaining
			}
			remain, err := api.chain.StateIndexProgress()
			if err == nil {
				prog.StateIndexRemaining = remain
			}
			return prog
		}
	)
	defer checkTimer.Stop()

	for {
		select {
		case i := <-api.installSyncSubscription:
			syncSubscriptions[i] = struct{}{}
			if done {
				i <- false
			}
		case u := <-api.uninstallSyncSubscription:
			delete(syncSubscriptions, u.c)
			close(u.uninstalled)
		case event := <-sub.Chan():
			if event == nil {
				return
			}
			switch event.Data.(type) {
			case StartEvent:
				started = true
			}
		case <-checkTimer.C:
			if !started {
				checkTimer.Reset(checkInterval)
				continue
			}
			prog := getProgress()
			if !prog.Done() {
				notification := &SyncingResult{
					Syncing: true,
					Status:  prog,
				}
				for c := range syncSubscriptions {
					c <- notification
				}
				checkTimer.Reset(checkInterval)
				continue
			}
			for c := range syncSubscriptions {
				c <- false
			}
			done = true
		}
	}
}

// Syncing provides information when this node starts synchronising with the Ethereum network and when it's finished.
func (api *DownloaderAPI) Syncing(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		statuses := make(chan interface{})
		sub := api.SubscribeSyncStatus(statuses)
		defer sub.Unsubscribe()

		for {
			select {
			case status := <-statuses:
				notifier.Notify(rpcSub.ID, status)
			case <-rpcSub.Err():
				return
			}
		}
	}()

	return rpcSub, nil
}

// SyncingResult provides information about the current synchronisation status for this node.
type SyncingResult struct {
	Syncing bool                  `json:"syncing"`
	Status  ethereum.SyncProgress `json:"status"`
}

// uninstallSyncSubscriptionRequest uninstalls a syncing subscription in the API event loop.
type uninstallSyncSubscriptionRequest struct {
	c           chan interface{}
	uninstalled chan interface{}
}

// SyncStatusSubscription represents a syncing subscription.
type SyncStatusSubscription struct {
	api       *DownloaderAPI   // register subscription in event loop of this api instance
	c         chan interface{} // channel where events are broadcasted to
	unsubOnce sync.Once        // make sure unsubscribe logic is executed once
}

// Unsubscribe uninstalls the subscription from the DownloadAPI event loop.
// The status channel that was passed to subscribeSyncStatus isn't used anymore
// after this method returns.
func (s *SyncStatusSubscription) Unsubscribe() {
	s.unsubOnce.Do(func() {
		req := uninstallSyncSubscriptionRequest{s.c, make(chan interface{})}
		s.api.uninstallSyncSubscription <- &req

		for {
			select {
			case <-s.c:
				// drop new status events until uninstall confirmation
				continue
			case <-req.uninstalled:
				return
			}
		}
	})
}

// SubscribeSyncStatus creates a subscription that will broadcast new synchronisation updates.
// The given channel must receive interface values, the result can either.
func (api *DownloaderAPI) SubscribeSyncStatus(status chan interface{}) *SyncStatusSubscription {
	api.installSyncSubscription <- status
	return &SyncStatusSubscription{api: api, c: status}
}
