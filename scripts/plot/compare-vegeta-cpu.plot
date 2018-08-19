set term svg
set output "bench/vegeta-CPU.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
plot \
    "bench/odatalite-vegeta/TOTALCPU-token.csv"  using 1:2 title "odatalite" with lines,  \
    "bench/sailfish-apache-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "bench/sailfish-https-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish" with lines, \
    "bench/sqlite-vegeta/TOTALCPU-token.csv"  using 1:2 title "sqlite" with lines
