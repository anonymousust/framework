global numOfStates; 
% cs = 0,1,2,2'
% la = 0,1
% lh = 0,1
% leader A/H = 0/1
numOfStates = 32;
global alphaPower;

% actions: 1 adopt, 2 wait, 3 release
choices = 3;
adopt = 1; wait = 2; release = 3;

global k;
delta=1;
Delta = delta*k;

global rou growth;
global P Bh T;

%%% transition
P = cell(1,choices);
Bh = cell(1,choices);
T = cell(1,choices);
growth = cell(1,choices);
for i = 1:choices
    P{i} = sparse(numOfStates, numOfStates);
    T{i} = sparse(numOfStates, numOfStates);
    Bh{i} = sparse(numOfStates, numOfStates);
    growth{i} = sparse(numOfStates, numOfStates);
end


H_H_time = 2*delta+Delta;
H_A_time = delta+2*Delta;
A_H_time = 3*Delta;
A_A_time = 3*Delta;


for state = 1:numOfStates
    [cs,la,lh,leader] = tchs_stnum2st(state);
    % next_cs denote result of cs+1
    if cs < 2
        next_cs = cs+1;
    elseif cs==2
        next_cs = 2;
    else % cs==3
        next_cs = 1;
    end
    
    % define adopt
    if leader == 0 % adopt-A
        if la == 0
            P{adopt}(state, tchs_st2stnum(cs,1,0,0)) = alphaPower;
            P{adopt}(state, tchs_st2stnum(cs,1,0,1)) = 1-alphaPower;
            Bh{adopt}(state, tchs_st2stnum(cs,1,0,0)) = lh;
            Bh{adopt}(state, tchs_st2stnum(cs,1,0,1)) = lh;
            T{adopt}(state, tchs_st2stnum(cs,1,0,0)) = A_A_time;
            T{adopt}(state, tchs_st2stnum(cs,1,0,1)) = A_H_time;
        else
            if cs == 2 || cs ==3
                cs_adopt_a1 = 3;
            else
                cs_adopt_a1 = 0;
            end
            P{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,0)) = alphaPower;
            P{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,1)) = 1-alphaPower;
            Bh{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,0)) = lh;
            Bh{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,1)) = lh;
            T{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,0)) = A_A_time;
            T{adopt}(state, tchs_st2stnum(cs_adopt_a1,1,0,1)) = A_H_time;
        end
    else % adopt-H
        if la == 0
            cs_adopt_h = next_cs;
        else
            cs_adopt_h = 1;
        end
        P{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = alphaPower;
        P{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = 1-alphaPower;
        Bh{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = lh;
        Bh{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = lh;
        T{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,0)) = H_A_time;
        T{adopt}(state, tchs_st2stnum(cs_adopt_h,0,1,1)) = H_H_time;
    end
    
    
    % define wait
    if leader == 0 % wait-A
        if la == 0
            if cs == 2 || cs ==3
                cs_wait_a0 = 3;
            else
                cs_wait_a0 = 0;
            end
            P{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,1)) = 1-alphaPower;
            T{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,0)) = A_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_a0,1,lh,1)) = A_H_time;
        else
            if lh > 0
                P{wait}(state, tchs_st2stnum(1,1,0,0)) = alphaPower;
                P{wait}(state, tchs_st2stnum(1,1,0,1)) = 1-alphaPower;
                T{wait}(state, tchs_st2stnum(1,1,0,0)) = A_A_time;
                T{wait}(state, tchs_st2stnum(1,1,0,1)) = A_H_time;
            else
                P{wait}(state, tchs_st2stnum(next_cs,1,0,0)) = alphaPower;
                P{wait}(state, tchs_st2stnum(next_cs,1,0,1)) = 1-alphaPower;
                T{wait}(state, tchs_st2stnum(next_cs,1,0,0)) = A_A_time;
                T{wait}(state, tchs_st2stnum(next_cs,1,0,1)) = A_H_time;
            end
        end
    else % wait - H
        if la == 0
            cs_wait_h = next_cs;
        else % la == 1
            cs_wait_h = 1;
        end
        if lh == 1
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = 1-alphaPower;
            Bh{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = 1;
            Bh{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = 1;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,0)) = H_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh,1)) = H_H_time;
        else
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,0)) = alphaPower;
            P{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,1)) = 1-alphaPower;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,0)) = H_A_time;
            T{wait}(state, tchs_st2stnum(cs_wait_h,0,lh+1,1)) = H_H_time;
        end
    end
    
    % define release
    if la == 1
        if leader == 0 % release-A
            if lh > 0
                cs_release_a = 1;
            else
                cs_release_a = next_cs;
            end
            P{release}(state, tchs_st2stnum(cs_release_a,1,0,0)) = alphaPower;
            P{release}(state, tchs_st2stnum(cs_release_a,1,0,1)) = 1-alphaPower;
            T{release}(state, tchs_st2stnum(cs_release_a,1,0,0)) = A_A_time;
            T{release}(state, tchs_st2stnum(cs_release_a,1,0,1)) = A_H_time;
        else % release-H
            P{release}(state, tchs_st2stnum(2,0,1,0)) = alphaPower;
            P{release}(state, tchs_st2stnum(2,0,1,1)) = 1-alphaPower;
            T{release}(state, tchs_st2stnum(2,0,1,0)) = H_A_time;
            T{release}(state, tchs_st2stnum(2,0,1,1)) = H_H_time;
        end
    else
        % for completeness
        P{release}(state, 1) = 1;
        Bh{release}(state, 1) = 10000;
        T{release}(state, 1) = A_A_time;
    end
    
end

disp(mdp_check(P, Bh))

epsilon = 0.0001;

lowRou = 0;
highRou = 1;
while(highRou - lowRou > epsilon/8)
    rou = (highRou + lowRou) / 2;
    for i = 1:choices
        growth{i} = (1-alphaPower-rou).*T{i} - Bh{i};
    end
    [growthPolicy, reward, cpuTime] = mdp_relative_value_iteration(P, growth, epsilon/8);
    if(reward > 0)
        lowRou = rou;
    else
        highRou = rou;
    end
end
disp('Chain growth: ')
format long
disp(1-alphaPower-rou)
