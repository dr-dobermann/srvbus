// SrvBus is an external service provider for
// BPMN v.2 compliant business process engine,
// implemented in gobpm package.
//
// (c) 2021, Ruslan Gabitov a.k.a. dr-dobermann.
// Use of this source is governed by LGPL license that
// can be found in the LICENSE file.

/*
Package srvbus united three servers:

  - EventServer -- in-memory pub/sub event server

  - MessageServer -- in-memory message server

  - ServiceServer -- in-memory services executor

Theese servers implemented simple functionality which are just enough as for
BMPN processes and as for many other cases.
They are not the best in their segment, but could simplify interfaces,
needed BMPN engine and other services.const.
If there is necessity to use another enterprise-grade server in particular area,
these servers could provide a proxy interface to use them.
*/
package srvbus
