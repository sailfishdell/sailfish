// Type: Periodic, OnChange, OnRequest
// 		Updates: AppendStopsWhenFull | AppendWrapsWhenFull | Overwrite | NewReport
//
// Periodic:   (*) AppendStopsWhenFull  (*) AppendWrapsWhenFull   (*) Overwrite   (*) NewReport
// OnChange:   (*) AppendStopsWhenFull  (*) AppendWrapsWhenFull   (?) Overwrite   (?) NewReport
// OnRequest:  (*) AppendStopsWhenFull  (*) AppendWrapsWhenFull   (?) Overwrite   (?) NewReport
//
// key:
//    (*) Done
// 		(-) makes sense and should be implemented
// 		(X) invalid combination, dont accept
//    (?) Not sure if this makes sense - more study needed
//
// AppendLimit: due to limitations in sqlite, this is a fixed limit that is a global setting that is applied when we create the VIEW
//
// behaviour:
//    Periodic: (on time interval, dump accumulated values into report. report can either be new/clean (for overwrite/newreport), or added to existing)
//      --> The Metric Value insert doesn't change reports. Best performance.
// 			--> Sequence is updated on period
// 		  --> Timestamp is updated on period
// 			--> For all reports: StartTimestamp and EndTimestamp are always fixed.
//      NewReport:  only keeps at most 3 reports, deletes oldest
//
//    OnRequest/OnChange:  things trickle in as they come
// 			AppendStopsWhenFull: StartTimestamp=fixed
// 			AppendWrapsWhenFull: StartTimestamp=fixed
//      NewReport: not supported
//      Overwrite: not supported

