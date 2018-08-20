set term svg
set output "vegeta-RPS.svg"
set xlabel "Requested rate"
set ylabel "Actual rate"
set autoscale
plot \
    "odatalite-vegeta/LATENCIES-token.csv" using 1:7 title "odatalite" with lines,    \
    "sailfish-apache-vegeta/LATENCIES-token.csv" using 1:7 title "sailfish-apache" with lines,    \
    "sailfish-https-vegeta/LATENCIES-token.csv" using 1:7 title "sailfish" with lines,    \
    "sqlite-vegeta/LATENCIES-token.csv" using 1:7 title "sqlite" with lines,    \
    "cim-vegeta/LATENCIES-token.csv" using 1:7 title "CIM" with lines
