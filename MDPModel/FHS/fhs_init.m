clear

global alphaPower;
global k;
global protocol;

% k=5/10/20/30/40

alphaPower = 0.3;

protocol = 0; % 1-tchs, others-FHS
k = 5;

disp(['alpha=',num2str(alphaPower)])
fhs_SolveStrategy_growth; 

fhs_SolveStrategy_latency;
