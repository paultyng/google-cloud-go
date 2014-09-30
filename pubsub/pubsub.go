// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package pubsub is a Google Cloud Pub/Sub client.
//
// More information about Google Cloud Pub/Sub is available on
// https://cloud.google.com/pubsub/docs
package pubsub

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	raw "code.google.com/p/google-api-go-client/pubsub/v1beta1"
)

const (
	// ScopePubSub grants permissions to view and manage Pub/Sub
	// topics and subscriptions.
	ScopePubSub = "https://www.googleapis.com/auth/pubsub"

	// ScopeCloudPlatform grants permissions to view and manage your data
	// across Google Cloud Platform services.
	ScopeCloudPlatform = "https://www.googleapis.com/auth/cloud-platform"
)

// Client is a Google Cloud Pub/Sub (Pub/Sub) client.
type Client struct {
	proj string
	s    *raw.Service
}

// SubClient represents a Pub/Sub subscription client.
type SubClient struct {
	proj string
	name string
	s    *raw.Service
}

// Topic represents a Pub/Sub topic client.
type TopicClient struct {
	proj string
	name string
	s    *raw.Service
}

// Message represents a Pub/Sub message.
type Message struct {
	// AckID is the identifier to acknowledge this message.
	AckID string

	// Data is the actual data in the message.
	Data []byte

	// Labels represents the key-value pairs the current message
	// is labelled with.
	Labels map[string]string
}

// New creates a new Pub/Sub client to manage topics and subscriptions
// under the provided project. The provided RoundTripper should be
// authorized and authenticated to make calls to Google Cloud Pub/Sub API.
// See the package examples for how to create an authorized http.RoundTripper.
func New(projID string, tr http.RoundTripper) *Client {
	return NewWithClient(projID, &http.Client{Transport: tr})
}

// NewWithClient creates a new Pub/Sub client to manage topics and
// subscriptions under the provided project. The client's
// Transport should be authorized and authenticated to make
// calls to Google Cloud Pub/Sub API.
// See the package examples for how to create an authorized http.RoundTripper.
func NewWithClient(projID string, c *http.Client) *Client {
	// TODO(jbd): Add user-agent.
	s, _ := raw.New(c)
	return &Client{proj: projID, s: s}
}

// TODO(jbd): Add subscription and topic listing.

// SubClient returns a client to perform operations on the
// subscription identified with the specified name.
func (c *Client) SubClient(name string) *SubClient {
	return &SubClient{
		proj: c.proj,
		name: name,
		s:    c.s,
	}
}

// Create creates a Pub/Sub subscription on the backend.
// A subscription should subscribe to an existing topic.
//
// The messages that haven't acknowledged will be pushed back to the
// subscription again when the default acknowledgement deadline is
// reached. You can override the default deadline by providing a
// non-zero deadline. Deadline must not be specified to
// precision greater than one second.
//
// As new messages are being queued on the subscription, you
// may recieve push notifications regarding to the new arrivals.
// To receive notifications of new messages in the queue,
// specify an endpoint callback URL.
// If endpoint is an empty string the backend will not notify the
// client of new messages.
//
// If the subscription already exists an error will be returned.
func (s *SubClient) Create(topic string, deadline time.Duration, endpoint string) error {
	sub := &raw.Subscription{
		Topic: fullTopicName(s.proj, topic),
		Name:  fullSubName(s.proj, s.name),
	}
	if int64(deadline) > 0 {
		if !isSec(deadline) {
			return errors.New("pubsub: deadline must not be specified to precision greater than one second")
		}
		sub.AckDeadlineSeconds = int64(deadline / time.Second)
	}
	if endpoint != "" {
		sub.PushConfig = &raw.PushConfig{PushEndpoint: endpoint}
	}
	_, err := s.s.Subscriptions.Create(sub).Do()
	return err
}

// Delete deletes the subscription.
func (s *SubClient) Delete() error {
	return s.s.Subscriptions.Delete(fullSubName(s.proj, s.name)).Do()
}

// ModifyAckDeadline modifies the current acknowledgement deadline
// for the messages retrieved from the current subscription.
// Deadline must not be specified to precision greater than one second.
func (s *SubClient) ModifyAckDeadline(deadline time.Duration) error {
	if !isSec(deadline) {
		return errors.New("pubsub: deadline must not be specified to precision greater than one second")
	}
	return s.s.Subscriptions.ModifyAckDeadline(&raw.ModifyAckDeadlineRequest{
		Subscription:       fullSubName(s.proj, s.name),
		AckDeadlineSeconds: int64(deadline),
	}).Do()
}

// ModifyPushEndpoint modifies the URL endpoint to modify the resource
// to handle push notifications coming from the Pub/Sub backend.
func (s *SubClient) ModifyPushEndpoint(endpoint string) error {
	return s.s.Subscriptions.ModifyPushConfig(&raw.ModifyPushConfigRequest{
		Subscription: fullSubName(s.proj, s.name),
		PushConfig: &raw.PushConfig{
			PushEndpoint: endpoint,
		},
	}).Do()
}

// Exists returns true if current subscription exists.
func (s *SubClient) Exists() (bool, error) {
	panic("not yet implemented")
}

// Ack acknowledges one or more Pub/Sub messages.
func (s *SubClient) Ack(id ...string) error {
	return s.s.Subscriptions.Acknowledge(&raw.AcknowledgeRequest{
		Subscription: fullSubName(s.proj, s.name),
		AckId:        id,
	}).Do()
}

// Pull pulls a new message from the subscription queue.
func (s *SubClient) Pull() (*Message, error) {
	return s.pull(true)
}

// PullWait pulls a new message from the subscription queue.
// If there are no messages left in the subscription queue, it will
// block until a new message arrives or timeout occurs.
func (s *SubClient) PullWait() (*Message, error) {
	return s.pull(false)
}

func (s *SubClient) pull(retImmediately bool) (*Message, error) {
	resp, err := s.s.Subscriptions.Pull(&raw.PullRequest{
		Subscription:      fullSubName(s.proj, s.name),
		ReturnImmediately: retImmediately,
	}).Do()
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(resp.PubsubEvent.Message.Data)
	if err != nil {
		return nil, err
	}

	labels := make(map[string]string)
	for _, l := range resp.PubsubEvent.Message.Label {
		if l.StrValue != "" {
			labels[l.Key] = l.StrValue
		} else {
			labels[l.Key] = strconv.FormatInt(l.NumValue, 10)
		}
	}
	return &Message{
		AckID:  resp.AckId,
		Data:   data,
		Labels: labels,
	}, nil
}

// TopicClient returns a topic client to run operations related to the Pub/Sub topics.
func (c *Client) TopicClient(name string) *TopicClient {
	return &TopicClient{
		proj: c.proj,
		name: name,
		s:    c.s,
	}
}

// Create creates a new topic with the current topic's name on the backend.
// It will return an error if topic already exists.
func (t *TopicClient) Create() error {
	_, err := t.s.Topics.Create(&raw.Topic{
		Name: fullTopicName(t.proj, t.name),
	}).Do()
	return err
}

// Delete deletes the current topic.
func (t *TopicClient) Delete() error {
	return t.s.Topics.Delete(fullTopicName(t.proj, t.name)).Do()
}

// Exists returns true if a topic named with the current topic's name exists.
func (t *TopicClient) Exists() (bool, error) {
	panic("not yet implemented")
}

// Publish publishes a new message to the current topic's subscribers.
// You don't have to label your message. Use nil if there are no labels.
// Label values could be either int64 or string. It will return an error
// if you provide a value of another kind.
func (t *TopicClient) Publish(data []byte, labels map[string]string) error {
	var rawLabels []*raw.Label
	if labels != nil {
		rawLabels := []*raw.Label{}
		for k, v := range labels {
			rawLabels = append(rawLabels, &raw.Label{Key: k, StrValue: v})
		}
	}
	return t.s.Topics.Publish(&raw.PublishRequest{
		Topic: fullTopicName(t.proj, t.name),
		Message: &raw.PubsubMessage{
			Data:  base64.StdEncoding.EncodeToString(data),
			Label: rawLabels,
		},
	}).Do()
}

// fullSubName returns the fully qualified name for a subscription.
// E.g. /subscriptions/project-id/subscription-name.
func fullSubName(proj, name string) string {
	return fmt.Sprintf("/subscriptions/%s/%s", proj, name)
}

// fullTopicName returns the fully qualified name for a topic.
// E.g. /topics/project-id/topic-name.
func fullTopicName(proj, name string) string {
	return fmt.Sprintf("/topics/%s/%s", proj, name)
}

func isSec(dur time.Duration) bool {
	return dur%time.Second == 0
}