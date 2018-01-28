   STRATEGY 1:
   -- SAGA A: listen for meta updated events. maintain a list of matching plugin lists.
   -- SAGA B: listen to events and emit commands - for events that are relevant to the plugin, traverse list of aggregates affected and emit commands to update the relevant aggregates


   STRATEGY 2:
   -- SAGA A: listen for events of interest and directly update the aggregate that you know should be updated (cmd: updateredfishresourceproperties)


   STRATEGY 3:
   -- Output plugin: grabs data when we ask to output it (least efficient way: discourage this for the most part.)

   I think for many things it would be best if we tended towards strategy 1, because we could write more generic code
