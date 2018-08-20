set term svg
set output "ab-LATENCIES.svg"
set xlabel "concurrent requests"
set ylabel "Median Latency"
set autoscale
plot  \
    "odatalite-ab/LATENCIES-token.csv"  using 1:4 title "odatalite" with lines,  \
    "sailfish-apache-ab/LATENCIES-token.csv"  using 1:4 title "sailfish-apache" with lines,  \
    "sailfish-https-ab/LATENCIES-token.csv"  using 1:4 title "sailfish" with lines, \
    "sqlite-ab/LATENCIES-token.csv"  using 1:4 title "sqlite" with lines
