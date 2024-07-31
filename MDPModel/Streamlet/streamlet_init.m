clear

global alphaPower;
global k;

% k=5/10/20/30/40

alphaPower = 1/3;
k = 5;

disp(['alpha=',num2str(alphaPower)])

streamlet_SolveStrategy_growth; 

streamlet_SolveStrategy_latency;
