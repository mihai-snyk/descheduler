#!/usr/bin/env python3
"""
Multi-Weight Configuration Comparison for Multi-Objective Kubernetes Descheduler

This script compares optimization results across different weight configurations,
showing how different weight profiles affect algorithm behavior and convergence.

Usage:
    python compare_weight_configurations.py <pattern>
    python compare_weight_configurations.py file1.json file2.json file3.json ...
    
Examples:
    python compare_weight_configurations.py "*.json"
    python compare_weight_configurations.py "*MixedWorkloads*.json"
    python compare_weight_configurations.py "optimization_results_*Cost*.json"
    python compare_weight_configurations.py file1.json file2.json file3.json
"""

import json
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
import pandas as pd
import sys
import os
import glob
import re
from pathlib import Path

# Set style
plt.style.use('seaborn-v0_8')
sns.set_palette("husl")

def expand_file_patterns(args):
    """Expand glob patterns and regex to actual file lists."""
    files = []
    
    for arg in args:
        if '*' in arg or '?' in arg:
            # Glob pattern
            matched_files = glob.glob(arg)
            files.extend(matched_files)
            print(f"🔍 Glob pattern '{arg}' matched {len(matched_files)} files")
        elif os.path.exists(arg):
            # Direct file
            files.append(arg)
        else:
            print(f"⚠️ Warning: '{arg}' not found")
    
    # Remove duplicates while preserving order
    unique_files = []
    seen = set()
    for f in files:
        if f not in seen:
            unique_files.append(f)
            seen.add(f)
    
    return unique_files

def load_multiple_optimization_data(filenames):
    """Load optimization results from multiple JSON files."""
    datasets = []
    for filename in filenames:
        if not os.path.exists(filename):
            print(f"Warning: File {filename} not found, skipping...")
            continue
        
        try:
            with open(filename, 'r') as f:
                data = json.load(f)
                # Add filename for reference
                data['filename'] = os.path.basename(filename)
                datasets.append(data)
        except Exception as e:
            print(f"Warning: Failed to load {filename}: {e}")
    
    return datasets

def extract_weight_profile_summary(weight_profile):
    """Create a simple summary of weight profile as tuple."""
    cost_w = weight_profile['Cost']
    disruption_w = weight_profile['Disruption'] 
    balance_w = weight_profile['Balance']
    
    # Just return the weight tuple
    return f"({cost_w:.1f}, {disruption_w:.1f}, {balance_w:.1f})"

def create_color_mapping(datasets):
    """Create consistent color mapping for weight configurations."""
    config_colors = {}
    # Use distinct colors for better distinction
    distinct_colors = [
        '#1f77b4',  # Blue
        '#ff7f0e',  # Orange  
        '#2ca02c',  # Green
        '#d62728',  # Red
        '#9467bd',  # Purple
        '#8c564b',  # Brown
        '#e377c2',  # Pink
        '#7f7f7f',  # Gray
        '#bcbd22',  # Olive
        '#17becf'   # Cyan
    ]
    
    color_idx = 0
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        if weight_summary not in config_colors:
            config_colors[weight_summary] = distinct_colors[color_idx % len(distinct_colors)]
            color_idx += 1
    
    return config_colors

def create_movement_comparison_by_disruption_weight(datasets):
    """Create movement comparison plots grouped by disruption weight."""
    fig, axes = plt.subplots(2, 2, figsize=(16, 12))
    
    # Create consistent color mapping for each unique weight configuration
    config_colors = create_color_mapping(datasets)
    
    # Plot 1: Total Movements 
    for data in datasets:
        rounds = []
        total_movements = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            total_movements.append(round_data['bestSolution']['movements'])
        
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        axes[0, 0].plot(rounds, total_movements, 'o-', 
                       label=weight_summary, 
                       color=color, linewidth=3, markersize=8)
    
    axes[0, 0].set_title('Total Movements per Round', fontweight='bold')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Number of Pod Movements')
    axes[0, 0].legend(loc='upper right', fontsize=9)
    axes[0, 0].grid(True, alpha=0.3)
    
    # Plot 2: Feasible Movements
    for data in datasets:
        rounds = []
        feasible_movements = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            if 'feasibleMoves' in round_data:
                feasible_movements.append(round_data['feasibleMoves']['feasibleMoves'])
            else:
                feasible_movements.append(round_data['bestSolution']['movements'])
        
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        axes[0, 1].plot(rounds, feasible_movements, 's-', 
                       label=weight_summary, 
                       color=color, linewidth=3, markersize=8)
    
    axes[0, 1].set_title('Feasible Movements per Round\n(Actually Executed)', fontweight='bold')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Number of Feasible Movements')
    axes[0, 1].legend(loc='upper right', fontsize=9)
    axes[0, 1].grid(True, alpha=0.3)
    
    # Plot 3: Feasibility Percentage
    for data in datasets:
        rounds = []
        feasibility_percent = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            if 'feasibleMoves' in round_data:
                feasibility_percent.append(round_data['feasibleMoves']['feasibilityPercent'])
            else:
                feasibility_percent.append(100.0)
        
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        axes[1, 0].plot(rounds, feasibility_percent, 'o-', 
                       label=weight_summary, 
                       color=color, linewidth=3, markersize=8)
    
    axes[1, 0].set_title('Movement Feasibility %\n(Target vs Actually Possible)', fontweight='bold')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Feasibility (%)')
    axes[1, 0].set_ylim(0, 105)
    axes[1, 0].axhline(y=100, color='green', linestyle='--', alpha=0.7, label='Perfect Feasibility')
    axes[1, 0].legend(loc='lower right', fontsize=9)
    axes[1, 0].grid(True, alpha=0.3)
    
    # Plot 4: Movement Convergence by Disruption Weight
    # Group datasets by disruption weight
    disruption_groups = {}
    for data in datasets:
        disruption_weight = data['testCase']['weightProfile']['Disruption']
        disruption_key = f"Disruption {disruption_weight:.1f}"
        
        if disruption_key not in disruption_groups:
            disruption_groups[disruption_key] = []
        disruption_groups[disruption_key].append(data)
    
    # Color palette for disruption weights (distinct, visible colors)
    disruption_color_palette = [
        '#1f77b4',  # Blue - Low disruption
        '#ff7f0e',  # Orange 
        '#2ca02c',  # Green
        '#d62728',  # Red
        '#9467bd',  # Purple
        '#8c564b',  # Brown - High disruption
    ]
    
    # Assign colors to disruption groups
    disruption_colors = []
    for i in range(len(disruption_groups)):
        disruption_colors.append(disruption_color_palette[i % len(disruption_color_palette)])
    
    for group_idx, (disruption_key, group_data) in enumerate(sorted(disruption_groups.items())):
        color = disruption_colors[group_idx]
        
        # Average movements across all configurations with same disruption weight
        all_rounds = []
        all_movements = []
        
        for data in group_data:
            for round_idx, round_data in enumerate(data['rounds']):
                all_rounds.append(round_idx + 1)
                all_movements.append(round_data['bestSolution']['movements'])
        
        if all_movements:
            # Group by round and calculate average
            round_movements = {}
            for round_num, movements in zip(all_rounds, all_movements):
                if round_num not in round_movements:
                    round_movements[round_num] = []
                round_movements[round_num].append(movements)
            
            rounds = sorted(round_movements.keys())
            avg_movements = [np.mean(round_movements[r]) for r in rounds]
            
            # Apply moving average for smoothness if enough data points
            if len(avg_movements) > 3:
                window_size = min(3, len(avg_movements))
                smoothed_movements = np.convolve(avg_movements, np.ones(window_size)/window_size, mode='valid')
                smoothed_rounds = rounds[window_size-1:]
                
                axes[1, 1].plot(smoothed_rounds, smoothed_movements, '-', 
                               label=disruption_key, 
                               color=color, linewidth=4, alpha=0.9)
            else:
                # For short sequences, plot raw averaged data
                axes[1, 1].plot(rounds, avg_movements, 'o-', 
                               label=disruption_key, 
                               color=color, linewidth=4, markersize=8, alpha=0.9)
    
    axes[1, 1].set_title('Movement Convergence by Disruption Weight\n(Average Across Same Disruption Weight)', fontweight='bold')
    axes[1, 1].set_xlabel('Round')
    axes[1, 1].set_ylabel('Average Movements')
    axes[1, 1].legend(loc='upper right', fontsize=10)
    axes[1, 1].grid(True, alpha=0.3)
    
    # Add text annotation about disruption effect
    axes[1, 1].text(0.02, 0.98, 'Higher disruption weight\n→ Fewer movements', 
                   transform=axes[1, 1].transAxes, fontsize=10, 
                   verticalalignment='top', bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.8))
    
    plt.suptitle('Movement Analysis Comparison Across Weight Configurations', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_actual_cost_evolution_comparison(datasets):
    """Compare actual resource cost evolution (starting from same point)."""
    fig, axes = plt.subplots(2, 2, figsize=(16, 12))
    
    # Create consistent color mapping
    config_colors = create_color_mapping(datasets)
    
    # Find the true initial cost (should be same for all configurations)
    true_initial_cost = None
    
    # Plot 1: Actual Resource Cost Evolution
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        rounds = []
        actual_costs = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            
            # Use bestSolution.rawCost for actual resource cost
            actual_cost = round_data['bestSolution']['rawCost']
            actual_costs.append(actual_cost)
            
            # Capture true initial cost from first round's initial state
            if round_idx == 0 and true_initial_cost is None:
                true_initial_cost = round_data['initialState']['totalCost']
        
        axes[0, 0].plot(rounds, actual_costs, 'o-', label=weight_summary, 
                       color=color, linewidth=3, markersize=8, alpha=0.8)
    
    # Add horizontal line for initial cost
    if true_initial_cost is not None:
        axes[0, 0].axhline(y=true_initial_cost, color='black', linestyle='--', 
                          alpha=0.7, linewidth=2, label=f'Initial Cost: ${true_initial_cost:.3f}/hr')
    
    axes[0, 0].set_title('Actual Resource Cost Evolution\n(All Start from Same Initial State)', fontweight='bold')
    axes[0, 0].set_xlabel('Round')
    axes[0, 0].set_ylabel('Total Resource Cost ($/hour)')
    axes[0, 0].legend(loc='upper right', fontsize=9)
    axes[0, 0].grid(True, alpha=0.3)
    
    # Plot 2: Cost Savings Accumulation
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        rounds = []
        cost_savings = []
        initial_cost = None
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            
            if round_idx == 0:
                initial_cost = round_data['initialState']['totalCost']
            
            actual_cost = round_data['bestSolution']['rawCost']
            savings = initial_cost - actual_cost if initial_cost else 0
            cost_savings.append(savings)
        
        axes[0, 1].plot(rounds, cost_savings, 's-', label=weight_summary, 
                       color=color, linewidth=3, markersize=8, alpha=0.8)
        axes[0, 1].fill_between(rounds, cost_savings, alpha=0.2, color=color)
    
    axes[0, 1].set_title('Cumulative Cost Savings\n(Relative to Initial State)', fontweight='bold')
    axes[0, 1].set_xlabel('Round')
    axes[0, 1].set_ylabel('Cost Savings ($/hour)')
    axes[0, 1].legend(loc='upper left', fontsize=9)
    axes[0, 1].grid(True, alpha=0.3)
    
    # Plot 3: Cost Reduction Percentage
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        rounds = []
        cost_reduction_percent = []
        initial_cost = None
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            
            if round_idx == 0:
                initial_cost = round_data['initialState']['totalCost']
            
            actual_cost = round_data['bestSolution']['rawCost']
            if initial_cost and initial_cost > 0:
                reduction_percent = ((initial_cost - actual_cost) / initial_cost) * 100
            else:
                reduction_percent = 0
            cost_reduction_percent.append(reduction_percent)
        
        axes[1, 0].plot(rounds, cost_reduction_percent, '^-', label=weight_summary, 
                       color=color, linewidth=3, markersize=8, alpha=0.8)
    
    axes[1, 0].set_title('Cost Reduction Percentage\n(% Improvement from Initial)', fontweight='bold')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Cost Reduction (%)')
    axes[1, 0].legend(loc='upper left', fontsize=9)
    axes[1, 0].grid(True, alpha=0.3)
    
    # Plot 4: Annual Savings Potential
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        rounds = []
        annual_savings = []
        initial_cost = None
        
        for round_idx, round_data in enumerate(data['rounds']):
            rounds.append(round_idx + 1)
            
            if round_idx == 0:
                initial_cost = round_data['initialState']['totalCost']
            
            actual_cost = round_data['bestSolution']['rawCost']
            hourly_savings = initial_cost - actual_cost if initial_cost else 0
            annual = hourly_savings * 24 * 365
            annual_savings.append(annual)
        
        axes[1, 1].plot(rounds, annual_savings, 'D-', label=weight_summary, 
                       color=color, linewidth=3, markersize=8, alpha=0.8)
    
    axes[1, 1].set_title('Annual Savings Potential\n(Extrapolated from Hourly)', fontweight='bold')
    axes[1, 1].set_xlabel('Round')
    axes[1, 1].set_ylabel('Annual Savings ($)')
    axes[1, 1].legend(loc='upper left', fontsize=9)
    axes[1, 1].grid(True, alpha=0.3)
    
    # Format y-axis for annual savings
    axes[1, 1].yaxis.set_major_formatter(plt.FuncFormatter(lambda x, p: f'${x:,.0f}'))
    
    plt.suptitle('Actual Resource Cost Evolution Comparison\n(Same Initial State, Different Optimization Focus)', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_objective_evolution_comparison(datasets):
    """Compare how different weight configurations affect objective evolution."""
    fig, axes = plt.subplots(2, 3, figsize=(20, 12))
    
    # Create consistent color mapping
    config_colors = create_color_mapping(datasets)
    
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        # Extract objective evolution
        rounds = []
        costs = []
        disruptions = []
        balances = []
        weighted_scores = []
        raw_costs = []
        raw_balances = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            best_sol = round_data['bestSolution']
            rounds.append(round_idx + 1)
            costs.append(best_sol['cost'])
            disruptions.append(best_sol['disruption'])
            balances.append(best_sol['balance'])
            weighted_scores.append(best_sol['weightedScore'])
            raw_costs.append(best_sol.get('rawCost', 0))
            raw_balances.append(best_sol.get('rawBalance', 0))
        
        # Plot normalized objectives
        axes[0, 0].plot(rounds, costs, 'o-', label=weight_summary, color=color, linewidth=3, markersize=6)
        axes[0, 1].plot(rounds, disruptions, 's-', label=weight_summary, color=color, linewidth=3, markersize=6)
        axes[0, 2].plot(rounds, balances, '^-', label=weight_summary, color=color, linewidth=3, markersize=6)
        
        # Plot raw metrics
        axes[1, 0].plot(rounds, raw_costs, 'o-', label=weight_summary, color=color, linewidth=3, markersize=6)
        axes[1, 1].plot(rounds, raw_balances, 's-', label=weight_summary, color=color, linewidth=3, markersize=6)
        
        # Plot weighted score
        axes[1, 2].plot(rounds, weighted_scores, '^-', label=weight_summary, color=color, linewidth=3, markersize=6)
    
    # Configure subplots
    axes[0, 0].set_title('Cost Evolution (Normalized)', fontweight='bold')
    axes[0, 0].set_ylabel('Cost (normalized)')
    axes[0, 0].grid(True, alpha=0.3)
    axes[0, 0].legend()
    
    axes[0, 1].set_title('Disruption Evolution (Normalized)', fontweight='bold')
    axes[0, 1].set_ylabel('Disruption (normalized)')
    axes[0, 1].grid(True, alpha=0.3)
    axes[0, 1].legend()
    
    axes[0, 2].set_title('Balance Evolution (Normalized)', fontweight='bold')
    axes[0, 2].set_ylabel('Balance (normalized)')
    axes[0, 2].grid(True, alpha=0.3)
    axes[0, 2].legend()
    
    axes[1, 0].set_title('Real Cost Evolution ($/hour)', fontweight='bold')
    axes[1, 0].set_xlabel('Round')
    axes[1, 0].set_ylabel('Cost ($/hour)')
    axes[1, 0].grid(True, alpha=0.3)
    axes[1, 0].legend()
    
    axes[1, 1].set_title('Real Balance Evolution (%)', fontweight='bold')
    axes[1, 1].set_xlabel('Round')
    axes[1, 1].set_ylabel('Balance (%)')
    axes[1, 1].grid(True, alpha=0.3)
    axes[1, 1].legend()
    
    axes[1, 2].set_title('Weighted Score Evolution', fontweight='bold')
    axes[1, 2].set_xlabel('Round')
    axes[1, 2].set_ylabel('Weighted Score (lower is better)')
    axes[1, 2].grid(True, alpha=0.3)
    axes[1, 2].legend()
    
    plt.suptitle('Objective Evolution Comparison Across Weight Configurations', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_2d_trajectory_comparison(datasets):
    """Compare final solution trajectories across weight configurations."""
    fig, axes = plt.subplots(1, 3, figsize=(20, 6))
    
    # Create consistent color mapping
    config_colors = create_color_mapping(datasets)
    
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        color = config_colors[weight_summary]
        
        # Extract final solutions from each round
        rounds = []
        costs = []
        disruptions = []
        balances = []
        
        for round_idx, round_data in enumerate(data['rounds']):
            best_sol = round_data['bestSolution']
            rounds.append(round_idx + 1)
            costs.append(best_sol['cost'])
            disruptions.append(best_sol['disruption'])
            balances.append(best_sol['balance'])
        
        # Plot trajectories
        axes[0].plot(costs, balances, 'o-', label=weight_summary, color=color, 
                    linewidth=3, markersize=8, alpha=0.9)
        axes[1].plot(costs, disruptions, 's-', label=weight_summary, color=color, 
                    linewidth=3, markersize=8, alpha=0.9)
        axes[2].plot(balances, disruptions, '^-', label=weight_summary, color=color, 
                    linewidth=3, markersize=8, alpha=0.9)
        
        # Mark start and end points
        if len(costs) > 1:
            axes[0].scatter([costs[0]], [balances[0]], c=[color], s=200, marker='o', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
            axes[0].scatter([costs[-1]], [balances[-1]], c=[color], s=200, marker='*', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
            
            axes[1].scatter([costs[0]], [disruptions[0]], c=[color], s=200, marker='o', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
            axes[1].scatter([costs[-1]], [disruptions[-1]], c=[color], s=200, marker='*', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
            
            axes[2].scatter([balances[0]], [disruptions[0]], c=[color], s=200, marker='o', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
            axes[2].scatter([balances[-1]], [disruptions[-1]], c=[color], s=200, marker='*', 
                          edgecolors='black', linewidth=2, alpha=0.9, zorder=10)
    
    # Configure subplots
    axes[0].set_xlabel('Cost (normalized)')
    axes[0].set_ylabel('Balance (normalized)')
    axes[0].set_title('Cost vs Balance Trajectories\n(○ = Start, ★ = End)', fontweight='bold')
    axes[0].legend()
    axes[0].grid(True, alpha=0.3)
    
    axes[1].set_xlabel('Cost (normalized)')
    axes[1].set_ylabel('Disruption (normalized)')
    axes[1].set_title('Cost vs Disruption Trajectories\n(○ = Start, ★ = End)', fontweight='bold')
    axes[1].legend()
    axes[1].grid(True, alpha=0.3)
    
    axes[2].set_xlabel('Balance (normalized)')
    axes[2].set_ylabel('Disruption (normalized)')
    axes[2].set_title('Balance vs Disruption Trajectories\n(○ = Start, ★ = End)', fontweight='bold')
    axes[2].legend()
    axes[2].grid(True, alpha=0.3)
    
    plt.suptitle('Solution Trajectory Comparison Across Weight Configurations', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def create_final_performance_comparison(datasets):
    """Compare final performance metrics across weight configurations."""
    fig, axes = plt.subplots(2, 2, figsize=(15, 10))
    
    # Create consistent color mapping
    config_colors = create_color_mapping(datasets)
    
    # Extract final performance data
    config_names = []
    final_costs = []
    final_disruptions = []
    final_balances = []
    final_weighted_scores = []
    final_raw_costs = []
    final_raw_balances = []
    total_movements = []
    
    for data in datasets:
        weight_summary = extract_weight_profile_summary(data['testCase']['weightProfile'])
        config_names.append(weight_summary)
        
        # Get final round data
        final_round = data['rounds'][-1]
        best_sol = final_round['bestSolution']
        
        final_costs.append(best_sol['cost'])
        final_disruptions.append(best_sol['disruption'])
        final_balances.append(best_sol['balance'])
        final_weighted_scores.append(best_sol['weightedScore'])
        final_raw_costs.append(best_sol.get('rawCost', 0))
        final_raw_balances.append(best_sol.get('rawBalance', 0))
        
        # Sum total movements across all rounds
        total_movements.append(sum(round_data['bestSolution']['movements'] 
                                 for round_data in data['rounds']))
    
    # Plot 1: Final Objective Values
    x_pos = np.arange(len(config_names))
    width = 0.25
    
    axes[0, 0].bar(x_pos - width, final_costs, width, label='Cost', alpha=0.8, color='steelblue')
    axes[0, 0].bar(x_pos, final_disruptions, width, label='Disruption', alpha=0.8, color='crimson')
    axes[0, 0].bar(x_pos + width, final_balances, width, label='Balance', alpha=0.8, color='forestgreen')
    
    axes[0, 0].set_title('Final Normalized Objective Values', fontweight='bold')
    axes[0, 0].set_ylabel('Objective Value (normalized)')
    axes[0, 0].set_xticks(x_pos)
    axes[0, 0].set_xticklabels(config_names, rotation=45, ha='right')
    axes[0, 0].legend()
    axes[0, 0].grid(True, alpha=0.3)
    
    # Plot 2: Final Real Metrics
    axes[0, 1].bar(x_pos - width/2, final_raw_costs, width, label='Cost ($/hour)', alpha=0.8, color='steelblue')
    ax_twin = axes[0, 1].twinx()
    ax_twin.bar(x_pos + width/2, final_raw_balances, width, label='Balance (%)', alpha=0.8, color='forestgreen')
    
    axes[0, 1].set_title('Final Real Metrics', fontweight='bold')
    axes[0, 1].set_ylabel('Cost ($/hour)', color='steelblue')
    ax_twin.set_ylabel('Balance (%)', color='forestgreen')
    axes[0, 1].set_xticks(x_pos)
    axes[0, 1].set_xticklabels(config_names, rotation=45, ha='right')
    axes[0, 1].grid(True, alpha=0.3)
    
    # Plot 3: Final Weighted Scores - Create colors explicitly
    score_colors = []
    for name in config_names:
        score_colors.append(config_colors[name])
    
    bars = axes[1, 0].bar(config_names, final_weighted_scores, alpha=0.8, color=score_colors)
    axes[1, 0].set_title('Final Weighted Scores\n(Lower = Better)', fontweight='bold')
    axes[1, 0].set_ylabel('Weighted Score')
    axes[1, 0].set_xticklabels(config_names, rotation=45, ha='right')
    axes[1, 0].grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, score in zip(bars, final_weighted_scores):
        height = bar.get_height()
        axes[1, 0].text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                       f'{score:.3f}', ha='center', va='bottom', fontsize=10, fontweight='bold')
    
    # Plot 4: Total Movement Activity - Create colors explicitly
    movement_colors = []
    for name in config_names:
        movement_colors.append(config_colors[name])
    
    bars = axes[1, 1].bar(config_names, total_movements, alpha=0.8, color=movement_colors)
    axes[1, 1].set_title('Total Movement Activity\n(Sum Across All Rounds)', fontweight='bold')
    axes[1, 1].set_ylabel('Total Pod Movements')
    axes[1, 1].set_xticklabels(config_names, rotation=45, ha='right')
    axes[1, 1].grid(True, alpha=0.3)
    
    # Add value labels on bars
    for bar, movements in zip(bars, total_movements):
        height = bar.get_height()
        axes[1, 1].text(bar.get_x() + bar.get_width()/2., height + height*0.01,
                       f'{movements}', ha='center', va='bottom', fontsize=10, fontweight='bold')
    
    plt.suptitle('Final Performance Comparison Across Weight Configurations', 
                 fontsize=16, fontweight='bold')
    plt.tight_layout()
    return fig

def main():
    if len(sys.argv) < 2:
        print("Usage: python compare_weight_configurations.py <pattern_or_files>")
        print()
        print("Examples:")
        print('  python compare_weight_configurations.py "*.json"')
        print('  python compare_weight_configurations.py "*MixedWorkloads*.json"')
        print('  python compare_weight_configurations.py "*Cost*" "*Balance*" "*Disruption*"')
        print("  python compare_weight_configurations.py file1.json file2.json file3.json")
        sys.exit(1)
    
    # Expand patterns to actual files
    json_files = expand_file_patterns(sys.argv[1:])
    
    if not json_files:
        print("❌ No JSON files found matching the provided patterns")
        sys.exit(1)
    
    print(f"📊 Loading optimization data from {len(json_files)} files...")
    datasets = load_multiple_optimization_data(json_files)
    
    if len(datasets) < 2:
        print("Error: Need at least 2 valid JSON files for comparison")
        sys.exit(1)
    
    print(f"✅ Loaded {len(datasets)} datasets for comparison:")
    for data in datasets:
        weight_profile = data['testCase']['weightProfile']
        print(f"   • {data['filename']}: {extract_weight_profile_summary(weight_profile)}")
    
    # Verify cluster consistency
    cluster_configs = set()
    for data in datasets:
        num_nodes = len(data['testCase']['nodes'])
        num_pods = len(data['testCase']['pods'])
        cluster_configs.add(f"{num_nodes}nodes_{num_pods}pods")
    
    if len(cluster_configs) > 1:
        print(f"⚠️ Warning: Multiple cluster configurations detected: {cluster_configs}")
        print("   Results may not be directly comparable")
    else:
        print(f"✅ All datasets use same cluster configuration: {list(cluster_configs)[0]}")
    
    # Create output directory
    output_dir = Path("weight_comparison_plots")
    output_dir.mkdir(exist_ok=True)
    
    print(f"🎨 Creating comparison visualizations...")
    
    # 1. Movement comparison
    fig = create_movement_comparison_by_disruption_weight(datasets)
    fig.savefig(output_dir / "01_movement_comparison.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 2. Actual cost evolution comparison (FIXED)
    fig = create_actual_cost_evolution_comparison(datasets)
    fig.savefig(output_dir / "02_actual_cost_evolution.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 3. Objective evolution comparison
    fig = create_objective_evolution_comparison(datasets)
    fig.savefig(output_dir / "03_objective_evolution_comparison.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 4. 2D trajectory comparison
    fig = create_2d_trajectory_comparison(datasets)
    fig.savefig(output_dir / "04_trajectory_comparison.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    # 5. Final performance comparison
    fig = create_final_performance_comparison(datasets)
    fig.savefig(output_dir / "05_final_performance_comparison.png", dpi=300, bbox_inches='tight')
    plt.close(fig)
    
    print(f"✅ All comparison visualizations saved to: {output_dir}/")
    print(f"📁 Files created:")
    for file in sorted(output_dir.glob("*.png")):
        print(f"   - {file.name}")
    
    print(f"\n🎯 Comparison Summary:")
    print(f"   • Datasets compared: {len(datasets)}")
    print(f"   • Weight configurations:")
    for data in datasets:
        weight_profile = data['testCase']['weightProfile']
        if data['rounds']:
            final_cost = data['rounds'][-1]['bestSolution'].get('rawCost', 0)
            final_movements = sum(round_data['bestSolution']['movements'] for round_data in data['rounds'])
            print(f"     - {extract_weight_profile_summary(weight_profile)}: "
                  f"Final cost=${final_cost:.3f}/hr, Total movements={final_movements}")

if __name__ == "__main__":
    main()
