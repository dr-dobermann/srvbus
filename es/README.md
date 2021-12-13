**es** is a simply in-memory Event Server.

# Introduction

**es** is a part of `srvbus` package - the Service Provider of the complex project [gobpm](https://github.com/dr-dobermann/gobpm) -- the BPMN v2. compliant run-time engine on Go. 

**Event Server** is a publisher/subscriber event server. Events stores in topics. Topics are organised in tree. Single levels of the branch are separated by '/' sign.

Every topic begins from the root and might look like `"/topic/subtopic/subsubtopic"`.

**Event Server** (es) is designed to use in a cooperation with the **Message Server** (ms) and the **Service Server** (s2), but it could be used separately in case there is only a necessity of the publishing events and.

For logging Event Server, like all others Servers in srvbus, uses [Uber zap logger](https://github.com/uber-go/zap).

### Events

Event has name, bytes data and event's firing time. For accessing to byte data, Event provides `Data()` method and implemented `io.Read` interface.

Events are stored in topics in `EventEnvelope`. EventEnvelope holds the publisher id, time of event registration on the Event Server and EventEnvelope id in topic's queue.

Events registered on the Event Server by `AddEvent` method. Target topic and sender should be provided with Event itself.

Events could be registered only on running server.

## Starting and Ending the Event Server

To create the Event Server just need to call `New` function. It takes name and id, but if they wasn't provided, new ones will be generated. `New` also demands SugraedLogger from uber's zap.

If there were no error, to start server run its `Run` method. This method takes context and flag of clean start. Event Server could be ended only by context's cancel function.

Event Server could rerun after its stopping. Just call `Run` method.

If clean start was choosen all previously created topics will be lost.

## Adding, processing and removing topics

Topics differs by their absolute names so there is no way to have two topics `/main` on the server. But there could easily coexist `/main/subtopic` and `/new/subtopic`.

Topic could be added only on running Event Server. Topics processing started by the Event Server that owns those topics while its `Run` method. If new topic added on running server its processing started after adding.

To add topic two methods could be used `AddTopic` and `AddTopicQueue`. `AddTopic` could add only one topic while `AddTopicQueue` could add the whole branch. Topics added to root or to a single existed topic.

To remove topic `RemoveTopic` method should be called. Topics could be removed reqursively.

## Subscribing to Events

Since events sends to topics, the subscription also made for the topic. Its possible to subscribe for events to whole branch from topic to the lowest subtopics. 

To subscribe for events `Subscribe` method of the Event Server should called.
This method could made a many subscription at once for a single subscriber.
One Subscription formed as `SubscrReq` -- subscription request. 

Event Server sends all appropriate events registered on the topic to a given EventEnvelope channel.

Every SubscrReq in addition to EventEnvelope channel has:

  - *Recursive* flag which make reqursive subscription for some or all underlying subtopics.

  - *Depth of recursion*. The subscriber could tell how deep should recurse the subscription.

  - *Event reading starting position*. Because all the events registered on the Event Server stores all the time since server runs, subscriber has ability to read not only events that occur after subscription, but all events on the topic since its creation. Server keeps the last nuber of Event it sent to the subscriber and supports their sequental sending.

  - *Filters* to filtrate events sending to the subscriber. Now implemented 4 filters:
    
    - `WithName` -- filters events with concrete name.

    - `WithSubName` -- filters events with name cosisted some substring.

    - `WithSubData` -- filters events with concrete byte sequence in the data.

    - `WithSubstr` -- filters events with concrete string in data.

Event Server don't close the EventEnvelope channel given while subscription registration. It's possible to use one EventEnvelope channel for many subscriptions.

Event Server don't check subscriptions for duplicates. If there are some, they would multiply event stream to the subscriber.

It is possible to make many subscription on the single topic.

## Removing subscription

To remove subscription `UnSubscribe` method of the EventServer should be called.

This removes all subscription on the topic for a single subscriber.

Since subscription doesn't have unique identification, its implssible to remove one subscription and left other intact.

