set term svg
set output "ab-CPU.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
set autoscale
plot \
    "odatalite-ab/TOTALCPU-token.csv"  using 1:2 title "odatalite" with lines,  \
    "sailfish-apache-ab/TOTALCPU-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "sailfish-https-ab/TOTALCPU-token.csv"  using 1:2 title "sailfish" with lines, \
    "sqlite-ab/TOTALCPU-token.csv"  using 1:2 title "sqlite" with lines
