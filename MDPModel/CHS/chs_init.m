clear

global alphaPower;
global k;

% k=5/10/20/30/40

alphaPower = 0.3;
k = 5;


disp(['alpha=',num2str(alphaPower)])

chs_SolveStrategy_growth; 

chs_SolveStrategy_latency;