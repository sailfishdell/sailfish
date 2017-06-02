Go Redfish - A redfish compliant server written in Go
===============

Proof of concept redfish server written in Go using the go text/template package.

Right now, it simply serves the DMTF mockup redfish targets. Exactly 0 features implemented!


TODO
====
    - authentication
    - authorization
    - 6.1.4 - Media Types: Compression support using Accept-Encoding header
    - 6.1.5 - etags
        - SHALL support ETAG for GET of ManagerAccount
        - PUT/PATCH should include ETAG in HTTP If-Match/If-None-Match header
        - ETAG: W/"<string>"
    - 6.3 A GET on the resource "/redfish" shall return the following body:
            { "v1": "/redfish/v1/" }

    - redfish-defined URIs
        - /redfish
        - /redfish/v1/
        - /redfish/v1/odata
        - /redfish/v1/$metadata

    - The /redfish/v1 should either redirect to /redfish/v1/ or it should be treated identically

    - HTTP Request Headers:
        - Accept (shall) (rfc7231) "Indicates to the server what media type(s) this
            client is prepared to accept. Services shall support
            requests for resources with an Accept header
            including application/json or application/
            json;charset=utf-8. Services shall support
            requests for metadata with an Accept header
            including application/xml or application/
            xml;charset=utf-8."
        - Accept-Encoding (should) (rfc7231)

    - Resource Retrieval: "Services may, but are not required to, support
            the convention of retrieving individual properties of a Resource by
            appending a segment containing the property name to the URI of the
            resource."

    - 6.4.2.4.1 - Query Parameters
        - $skip
        - $top

    - extended errors

    - 6.4.4.1 - PATCH -

    - HTTP Response Headers:


placeholder note:
Need to go over the notes in this blog post to make sure this server is up to snuff:
https://blog.gopheracademy.com/advent-2016/exposing-go-on-the-internet/
