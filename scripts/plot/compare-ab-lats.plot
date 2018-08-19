set term svg
set output "bench/ab-LATENCIES.svg"
set xlabel "concurrent requests"
set ylabel "Latency"
plot  \
    "bench/odatalite-ab/LATENCIES-token.csv"  using 1:4 title "odatalite" with lines,  \
    "bench/sailfish-apache-ab/LATENCIES-token.csv"  using 1:4 title "sailfish-apache" with lines,  \
    "bench/sailfish-https-ab/LATENCIES-token.csv"  using 1:4 title "sailfish" with lines, \
    "bench/sqlite-ab/LATENCIES-token.csv"  using 1:4 title "sqlite" with lines
