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





		//   Periodic,  AppendWraps:     ++ 	Now()            			-- (don't update)
		//   Periodic,  AppendStops:     ++ 	Now()            			-- (don't update)
		//   Periodic,  NewReport  :          - never udpated 		  -
		//   Periodic,  Overwrite  :     ++ 	Now() 					  		Prev ReportTimestamp
		//   OnChange,  AppendWraps:     ++ 	Now()             		-- (don't update)
		//   OnChange,  AppendStops:     ++ 	Now()             		-- (don't update)
		//   OnChange,  NewReport  :          - never udpated 		  -
		//   OnChange,  Overwrite  :     ++ 	Now() 								Prev ReportTimestamp
		//   OnRequest, IMPLICIT   :  		    - never updated 		  -
		//
		// SQL List:
		//   - Set Start to ReportTimestamp
		//   - Update ReportTS
		// 	 - Update SEQ

		// query:
		//   Periodic,  AppendWraps:    seq 	End=ReportTimeStamp   Start=Start
		//   Periodic,  AppendStops:    seq 	End=ReportTimeStamp   Start=Start
		//   Periodic,  NewReport  :    seq 	End=ReportTimeStamp   Start=Start
		//   Periodic,  Overwrite  :    seq 	End=ReportTimeStamp   Start=Start
		//   OnChange,  AppendWraps:    seq 	End=ReportTimeStamp   Start=End - TimeSpan
		//   OnChange,  AppendStops:    seq 	End=ReportTimeStamp   Start=End - TimeSpan
		//   OnChange,  NewReport  :    seq 	End=ReportTimeStamp   Start=End - TimeSpan
		//   OnChange,  Overwrite  :    seq 	End=ReportTimeStamp   Start=End - TimeSpan
		//   OnRequest, IMPLICIT   : 		seq 	End=Now() 						Start=End - TimeSpan

