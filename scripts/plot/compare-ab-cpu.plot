set term svg
set output "bench/ab-CPU.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
plot \
    "bench/odatalite-ab/TOTALCPU-token.csv"  using 1:2 title "odatalite" with lines,  \
    "bench/sailfish-apache-ab/TOTALCPU-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "bench/sailfish-https-ab/TOTALCPU-token.csv"  using 1:2 title "sailfish" with lines, \
    "bench/sqlite-ab/TOTALCPU-token.csv"  using 1:2 title "sqlite" with lines
