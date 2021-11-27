**ms** is a simply in-memory Message Server.

# Introduction

**ms** is a part of `srvbus` package - the Service Provider of the complex project [gobpm](https://github.com/dr-dobermann/gobpm) -- the BPMN v2. compliant run-time engine on Go. 

**Message Server** (ms) is designed to use in a cooperation with the **Event Server** (es) and the **Service Server** (s2), but it could be used separately in case there is only a necessity of the queued messages interchange.

For logging Message Server uses [Uber zap logger](https://github.com/uber-go/zap).

## Running the Message Server

The Message Server has a very simple API. To create a new Message Server just call `New` function of the package `ms`. It takes server id, its name and pointer to the sugared zap logger.

if logger isn't presented then error would be returned. If id or name weren't given, they will be created automatically.

Once the server is created it should be run with `Run` method with appropriate context. Run checks if the server is already runned. If so it's just returns back.

In case the Server is stopped early, all the queues created in the previous session will be deleted before new run.

After the server is created its possible to Put messages into it and Get messages out of it.

## Putting messages to the server

To put messages `PutMessages` method of MessageServer should be invoked. Current realization doesn't provide direct queues management. When messages are putting into the server, the name of queue should be given. If there is no queue with the given name, then the new queue will be created. When messages are putting into the queue, the sender id of these messages should be provided.

`PutMessages` consumes variadic number of messages and all of them are storing into the same queue in the FIFO order. 

## Getting messages from the server

To get saved on the Message Server messages, `GetMessages` method should be called. This method returns a list of `MessageEnvelopes` which consists among the Message itself also time of it registration on the Server and the ID of the message sender. 

`GetMessages` demands the receiver ID, and the queue name. If there were previous reading from the same queue and for the same receiver, then only new messages would be returned. To read queue from the start,
the third parameter `fromBegin` shold be set to `true`.

If there are no queue the receiver are asking messages from, then error will be returned.

## Getting a list of the queues existed on the server

To get a list of queues existed on the server the method `Queues` should be invoked. It returns a slice of the `QueueStat` structs which consist queue name and the number of messages in the queue. If the number of message is -1, then the queue's processing is stopped and it couldn't put into or get out messages.

In the present moment I don't see any neccessity to provide a queue's restarting tools, but it could be easily added if someone provides a good reason for it.

## Stopping the server

To stop the Message Server user should invoke `cancel` function of the context. Server stops processing of all the registered queues.

## Message

Messages only has three fields
  
    Message struct {
      id  uuid.UUID
      Key string
      data []bytes
    }

Method `ID` returns an Id of the Message

The maximum size of data couldn't exceed 8K bytes (8192 bytes). 

There are two functions for creating Message object:

    func NewMsg(
        id uuid.UUID, 
        key string,
        r io.Reader) (*Message, error)

    func GetMsg(
        id uuid.UUID,
        key string,
        r io.Reader) *Message

The only difference betweet theese two is that the latter one returns only the Message pointer and if there is any error, it panics.

If the message Id isn't presented (user send `uuid.Nil` as a parameter) the new one will be generated.

The data field of the message stores from the given `io.Reader r`.
To access stored data Message provides two ways:

  1. Use `Data` method which returns []byte.

  2. Use `io.Reader` interface implemented for the Message.
