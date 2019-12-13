
#include <stdio.h>
#include <libdds.h>


int32_t wd_timer( zloop_t *reactor, int32_t timer_id, void *user_data );
int32_t first_timer( zloop_t *reactor, int32_t timer_id, void *user_data );

#define DAEMON_COMPONENT_NAME   "metric-engine"
//#define DAEMON_DDSC_FLAGS       ( DDSC_FLG_CLIENT | DDSC_FLG_PROVIDE | DDSC_FLG_ZLOOP | DDSC_FLG_SUBSCRIBE )
#define DAEMON_DDSC_FLAGS       ( DDSC_FLG_CLIENT | DDSC_FLG_ZLOOP | DDSC_FLG_SUBSCRIBE )

// daemon info, there can be only one...
static dds_daemon_t daemon_info = {
    DAEMON_COMPONENT_NAME,       // Provider/publisher name, must be unique
    0,                           // delldebug component
    DAEMON_DDSC_FLAGS
};


// function provider table, must end in a NULL entry
static dds_function_provider_t daemon_fp_tbl[] = {
    // must be last
    { NULL, NULL, 0 }
};

// zloop timers.  The table ends with an entry with NULL handler function.
static dds_zlt_t daemon_dz_tbl[] = {
    { wd_timer, 2000, 0 },
    { first_timer, 0, 1 },         // only run once

    { NULL, 0, 0 }
};

// FP and socket watch table, must end in NULL
static dds_zlp_t daemon_zlp_tbl[] = {
    // must be last
    { NULL, NULL, 0, 0 }
};

dds_notification_topic_t daemon_dns_tbl[] = {
    // last
    { 0 }
};

static dds_input_config_attr_t ec_config_attr_tbl[] = {
    // last
    {0}
};

int32_t wd_timer( zloop_t *reactor, int32_t timer_id, void *user_data )
{
printf("wd_timer!\n");
return 0;
}


// init timer
int32_t first_timer( zloop_t *reactor, int32_t timer_id, void *user_data )
{
printf("first_timer!\n");
return 0;
}


int start_cgo_event_loop() {
    int status = 0;
    dds_input_options_t daemon_options = {0};
    dds_context_t *daemon_ddsc = NULL;

    printf("\n\nHello from C world (again)!\n\n");

    // dds input options intilization
    memset(&daemon_options, 0, sizeof(dds_input_options_t));
    daemon_options.ddsi_daemon_info = &daemon_info;
    daemon_options.ddsi_flags = daemon_info.dsd_ddsc_flags;
    daemon_options.ddsi_fp_tbl = daemon_fp_tbl;
    daemon_options.ddsi_zlp_tbl = daemon_zlp_tbl;
    daemon_options.ddsi_zlt_tbl = daemon_dz_tbl;
    daemon_options.ddsi_topic_tbl = daemon_dns_tbl;
    daemon_options.ddsi_cluster_callback = NULL;

    /*
    if (isDeviceEC()) {
        daemon_options.ddsi_dca_tbl = ec_config_attr_tbl;
        daemon_options.ddsi_config_attr_callback = config_attr_callback;
        daemon_options.ddsi_flags |= DDSC_FLG_CONFIG_ATTR;
        daemon_info.dsd_ddsc_flags |= DDSC_FLG_CONFIG_ATTR;
    }
    */

    status = ddsc_initialize(&daemon_ddsc, &daemon_options);
    if ( status ) {
        printf("DDS status %i\n", status);
        return 3;
    }

    daemon_ddsc->ddsc_user_info = NULL;

    if (daemon_ddsc->ddsc_reactor) {
        zloop_start( daemon_ddsc->ddsc_reactor );
    }

    /*
    printf("shutting down subscriber\n");
    if (ctx->sub_context) {
        fnmgr_subscriber_destroy(&ctx->sub_context);
    }
    */

    printf("destroying dds context\n");
    ddsc_destroy( daemon_ddsc );

    return 0;
}
