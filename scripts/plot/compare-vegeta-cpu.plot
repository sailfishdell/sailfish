set term svg
set output "vegeta-CPU.svg"
set xlabel "concurrent requests"
set ylabel "Total CPU utilization"
set autoscale
plot \
    "odatalite-vegeta/TOTALCPU-token.csv"  using 1:2 title "odatalite" with lines,  \
    "sailfish-apache-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish-apache" with lines,  \
    "sailfish-https-vegeta/TOTALCPU-token.csv"  using 1:2 title "sailfish" with lines, \
    "sqlite-vegeta/TOTALCPU-token.csv"  using 1:2 title "sqlite" with lines,    \
    "cim-vegeta/TOTALCPU-token.csv"  using 1:2 title "CIM" with lines
