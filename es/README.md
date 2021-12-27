**es** is a simply in-memory Event Server.

# Introduction

**es** is a part of `srvbus` package - the Service Provider for the complex project [gobpm](https://github.com/dr-dobermann/gobpm) -- the BPMN v2. compliant run-time engine on Go. 

**Event Server** is a publisher/subscriber event server. Events stores in topics. Topics are organised in tree. Single levels of the branch are separated by '/' sign.

Every topic begins from the root and might look like `"/topic/subtopic/subsubtopic"`.

**Event Server** (es) is designed to use in a cooperation with the [**Message Server** (ms)](https://github.com/dr-dobermann/srvbus/tree/master/ms) and the [**Service Server** (s2)](https://github.com/dr-dobermann/srvbus/tree/master/s2), but it could be used separately in case there is only a necessity of the publishing events and.

For logging Event Server, like all others Servers in srvbus, uses [Uber zap logger](https://github.com/uber-go/zap).

## Events and Event Envelopes

Event has string name, []byte data and event's firing time. For accessing to byte data, Event provides `Data()` method and `io.Read` interface.

Events are stored in topics in `EventEnvelope`. EventEnvelope holds the publisher id, queue name, time of event registration on the Event Server and EventEnvelope id in topic's queue.

Events registered on the Event Server by `AddEvent` method. Events could be registered only on running EventServer in other case an error will be returned.

# Event Server

## Starting and Ending the Event Server

To create the EventServer just need to call `New` function of `es` package. It takes name and id, but if they weren't provided, new ones will be generated. `New` also demands SugraedLogger from uber's zap.

If there were no error, to start server call its `Run` method. This method takes context and flag of clean start. Event Server could be rerun after its stopping. Just call `Run` method again with `cleanStart` false. If `cleanStart` was set to true, all previously created topics will be lost. 

On the start or clean run of the Event Server internal topic `/server` is created. It takes internal events of the server: creating `TOPIC_CREATED_EVT` and deleting `TOPIC_DELETED_EVT` topics, open `SUBSCRIBED_EVT` and cancel `UNSUBSCRIBED_EVT` subscription. In case of necessity of tracking such events, just subscribe for the `/server` topic and get interesting events.

Event Server could be ended only by context's cancel function given by `Run` calling. So don't use `context.Background` on the call.

## gRPC interface of the EventServer

EventServer has a gRPC interface. It could be accessed from `api/grpc/esgrpc` package. Package provides simple gRPC hadler which created by `New` call. Just send to it pointer of the EventServer and optional zap.SugaredLogger (if the logger is nil, the hosted MessageServer logger will be used.).

After creating the EventServer's gRPC handler, just run it with its `Run` method using appropriate context.

Clients of the gRPC could access followed functions:

    rpc HasTopic(TopicRequest) returns (OpResponse) {}

    rpc AddTopics(AddTopicReq) returns (OpResponse) {}

    rpc DelTopics(DelTopicReq) returns (OpResponse) {}

    rpc AddEvent(EventRegistration) returns (OpResponse) {}

    rpc Subscribe(SubscriptionRequest) returns (stream EventEnvelope) {}

    rpc UnSubscribe(UnsubsibeRequest) returns (OpResponse) {}

    rpc StopSubscriptionStream(StopStreamRequest) returns (OpResponse) {}

All rpc's demands the server_id as a field of a request. It's just an EventServer ID. User could take it from the id used on `New` call, `ID` call, gRPC handler logs.

Using of `HasTopic, AddTopics, DelTopics, AddEvent` is trivial and practically is not differ from the EventServer's methods. But subscription managing demands to take into account a gRPC specifics.

Subscribe demands a `subs_stream_id` as request field. It is unique id of the _gRPC stream_ **which should be closed directly**. `Unsubscribe` only closes the sending of EventEnvelopes into the subscription channel.

To close gRPC's EventEnvelope stream, `StopSubscriptionStream` should be called.

## Adding, processing and removing topics

Topics differs by their absolute names so there is no way to have two topics `/main` on the server. But there could easily coexist `/main/subtopic` and `/new/subtopic`.

Topic could be added only on running Event Server. Topics processing started by the Event Server that owns those topics while its `Run` method. If new topic added on running server its processing started after adding.

To add topic two methods could be used `AddTopic` and `AddTopicQueue`. `AddTopic` could add only one topic while `AddTopicQueue` could add the whole branch. Topics added to root or to a single existed topic.

To remove topic `RemoveTopic` method should be called. Topics could be removed reqursively.

## Subscribing to Events

Since events sends to topics, the subscription also made for the topic. Its possible to subscribe for events to whole topic's branch from topic to the underlayed subtopics. 

To subscribe for events `Subscribe` method of the Event Server should called.
This method could made a many subscription at once for a single subscriber.
One Subscription formed as `SubscrReq` -- subscription request. 

Event Server sends all appropriate events registered on the topic to a given EventEnvelope channel.

Every SubscrReq in addition to EventEnvelope channel has:

  - *Recursive* flag which make reqursive subscription for some or all underlying subtopics.

  - *Depth of recursion*. The subscriber could tell how deep should recursion continues for the levels of subtopics.

  - *Event reading starting position*. Because all the events registered on the Event Server are stored all since the server runs, subscriber has ability to read not only events that occur after the  subscription, but all the events on the topic since its creation. Server keeps the last number of the Event it sent to the subscriber and supports their sequental sending if there is no filters used in subscription request. Using filters could change the sequence of thi events passing to the subscriber.

  - *Filters* to filtrate events sending to the subscriber. Now implemented 4 filters:
    
    - `WithName` -- filters events with concrete name.

    - `WithSubName` -- filters events with name cosisted some substring.

    - `WithSubData` -- filters events with concrete byte sequence in the data.

    - `WithSubstr` -- filters events with concrete string in data.

Event Server don't close the EventEnvelope channel given while subscription registration. It's possible to use one EventEnvelope channel for many subscriptions, so EventServer has no information about channel usage.

Event Server don't check subscriptions for duplicates. If there are some, they would multiply event stream to the subscriber.

It is possible to make many subscription on the single topic.

## Removing subscription

To remove subscription `UnSubscribe` method of the EventServer should be called.

This removes all subscription on the topic for a single subscriber.

Since subscription doesn't have unique identification, its implssible to remove one subscription and left other intact for a single topic and a subscriber.

