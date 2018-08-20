set term svg
set output "vegeta-CPU-0-30.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
set xrange [0:30]
set yrange [0:100]
plot \
    "odatalite-vegeta/TOTALCPU-token.csv"  using 1:2 title "odatalite" with lines,  \
    "sailfish-apache-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "sailfish-https-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish" with lines, \
    "sqlite-vegeta/TOTALCPU-token.csv"  using 1:2 title "sqlite" with lines
