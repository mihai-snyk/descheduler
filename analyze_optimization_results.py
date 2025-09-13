#!/usr/bin/env python3
"""
Multi-Objective Optimization Results Analysis and Visualization

This script analyzes JSON output from the NSGA-II multiobjective optimization tests
and creates publication-quality visualizations for academic papers.

Usage:
    python analyze_optimization_results.py optimization_results_*.json

Features:
- Convergence analysis plots
- Pareto front evolution visualization  
- Algorithm comparison (NSGA-II vs baselines)
- Trade-off analysis with parallel coordinates
- Statistical significance testing
- Performance benchmarking
"""

import json
import sys
import glob
import argparse
from pathlib import Path
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import seaborn as sns
from typing import List, Dict, Any, Tuple, Optional
import warnings
warnings.filterwarnings('ignore')

# Set matplotlib style for academic papers
plt.style.use('seaborn-v0_8-whitegrid')
plt.rcParams.update({
    'font.size': 12,
    'axes.labelsize': 14,
    'axes.titlesize': 16,
    'xtick.labelsize': 11,
    'ytick.labelsize': 11,
    'legend.fontsize': 11,
    'figure.titlesize': 18,
    'figure.dpi': 300,
    'savefig.dpi': 300,
    'savefig.bbox': 'tight',
    'savefig.format': 'png'
})

class OptimizationAnalyzer:
    """Analyzes multi-objective optimization results from JSON files."""
    
    def __init__(self, json_files: List[str]):
        """Initialize analyzer with list of JSON result files."""
        self.json_files = json_files
        self.results = []
        self.load_results()
        
    def load_results(self):
        """Load all JSON result files."""
        print(f"Loading {len(self.json_files)} result files...")
        
        for json_file in self.json_files:
            try:
                with open(json_file, 'r') as f:
                    data = json.load(f)
                    data['source_file'] = json_file
                    self.results.append(data)
                    print(f"  ✓ Loaded {json_file}")
            except Exception as e:
                print(f"  ✗ Failed to load {json_file}: {e}")
                
        print(f"Successfully loaded {len(self.results)} result files\n")
    
    def create_all_visualizations(self, output_dir: str = "optimization_analysis"):
        """Create all visualization plots and save to output directory."""
        output_path = Path(output_dir)
        output_path.mkdir(exist_ok=True)
        
        print(f"Creating visualizations in {output_path}/...")
        
        # 1. Convergence Analysis
        self.plot_convergence_analysis(output_path)
        
        # 2. Pareto Front Evolution
        self.plot_pareto_front_evolution(output_path)
        
        # 3. Algorithm Comparison
        self.plot_algorithm_comparison(output_path)
        
        # 4. Trade-off Analysis
        self.plot_tradeoff_analysis(output_path)
        
        # 5. Performance Benchmarking
        self.plot_performance_comparison(output_path)
        
        # 6. Statistical Analysis
        self.plot_statistical_analysis(output_path)
        
        # 7. Scalability Analysis (if multiple scenarios)
        if len(self.results) > 1:
            self.plot_scalability_analysis(output_path)
            
        print(f"\n🎨 All visualizations saved to {output_path}/")
    
    def plot_academic_bar_comparison(self, output_path: Path):
        """Create academic-style grouped bar charts comparing algorithms across different node counts."""
        # Group results by node count
        scenario_data = {}
        
        for result in self.results:
            test_case = result['testCase']
            node_count = len(test_case['nodes'])
            
            # Use node count as the x-axis variable
            if node_count not in scenario_data:
                scenario_data[node_count] = {
                    'nsga_ii': {},
                    'baselines': {}
                }
            
            # Extract NSGA-II BEST results across ALL rounds (not just final round)
            if 'rounds' in result and result['rounds']:
                # Find absolute best values across all rounds
                best_cost = float('inf')
                best_balance = float('inf')
                best_disruption = float('inf')
                best_cost_round = 0
                best_balance_round = 0
                best_disruption_round = 0
                
                for i, round_data in enumerate(result['rounds']):
                    solution = round_data['bestSolution']
                    
                    if solution['rawCost'] < best_cost:
                        best_cost = solution['rawCost']
                        best_cost_round = i + 1
                    
                    if solution['rawBalance'] < best_balance:
                        best_balance = solution['rawBalance']
                        best_balance_round = i + 1
                        
                    if solution['disruption'] < best_disruption:
                        best_disruption = solution['disruption']
                        best_disruption_round = i + 1
                
                print(f"  Node count {node_count}: NSGA-II best values found in:")
                print(f"    Best cost: {best_cost:.4f} (round {best_cost_round})")
                print(f"    Best balance: {best_balance:.4f} (round {best_balance_round})")
                print(f"    Best disruption: {best_disruption:.4f} (round {best_disruption_round})")
                
                scenario_data[node_count]['nsga_ii'] = {
                    'cost': best_cost,  # Absolute best cost across all rounds
                    'balance': best_balance,  # Absolute best balance across all rounds
                    'disruption': best_disruption,  # Absolute best disruption across all rounds
                    'rounds_completed': len(result['rounds'])
                }
            
            # Extract baseline results  
            if 'baselineResults' in result:
                for baseline in result['baselineResults']:
                    alg_name = baseline['algorithm']
                    if alg_name not in scenario_data[node_count]['baselines']:
                        scenario_data[node_count]['baselines'][alg_name] = {}
                    
                    scenario_data[node_count]['baselines'][alg_name] = {
                        'cost': baseline['rawCost'],
                        'balance': baseline['rawBalance'], 
                        'disruption': 0,  # Baselines don't move pods, they do initial placement
                        'execution_time': baseline.get('executionTimeMs', 0) / 1000.0
                    }
        
        if len(scenario_data) < 2:
            print("  ⚠ Not enough scenarios for academic-style comparison (need at least 2 different node counts)")
            return
            
        # Create grouped bar chart
        fig, axes = plt.subplots(1, 2, figsize=(14, 7))
        fig.suptitle('Algorithm Performance Comparison: Grouped by Cluster Size', fontsize=16, fontweight='bold')
        
        # Sort node counts for consistent x-axis
        node_counts = sorted(scenario_data.keys())
        print(f"📊 Creating grouped bar comparison across {len(node_counts)} cluster sizes: {node_counts}")
        
        # Define algorithm colors and create mapping for short names
        algorithm_name_mapping = {
            'Best Fit Decreasing': 'BFD',
            'First Fit Decreasing': 'FFD', 
            'Random Placement': 'Random',
            'Greedy Cost Scheduler': 'Greedy',
            'NSGA-II (Proposed)': 'NSGA-II'
        }
        
        algorithm_colors = {
            'NSGA-II': '#1f77b4',
            'BFD': '#ff7f0e',
            'FFD': '#2ca02c',
            'Random': '#d62728',
            'Greedy': '#9467bd'
        }
        
        # Prepare data for grouped bars
        objectives = ['cost', 'balance']
        objective_labels = ['Cost (USD/hour)', 'Load Balance (%)']
        
        for i, (obj, label) in enumerate(zip(objectives, objective_labels)):
            ax = axes[i]
            
            # Collect all algorithm names and convert to short names
            all_algorithms = set(['NSGA-II'])
            for nc in node_counts:
                if 'baselines' in scenario_data[nc]:
                    # Convert baseline names to short names
                    baseline_short_names = [algorithm_name_mapping.get(name, name) for name in scenario_data[nc]['baselines'].keys()]
                    all_algorithms.update(baseline_short_names)
            
            all_algorithms = sorted(list(all_algorithms))
            
            # Prepare data for each algorithm
            algorithm_data = {}
            for alg in all_algorithms:
                algorithm_data[alg] = []
                for nc in node_counts:
                    if alg == 'NSGA-II':
                        if 'nsga_ii' in scenario_data[nc] and obj in scenario_data[nc]['nsga_ii']:
                            algorithm_data[alg].append(scenario_data[nc]['nsga_ii'][obj])
                        else:
                            algorithm_data[alg].append(0)
                    else:
                        # Find original baseline name from short name
                        original_name = None
                        for orig, short in algorithm_name_mapping.items():
                            if short == alg:
                                original_name = orig
                                break
                        
                        if (original_name and original_name in scenario_data[nc].get('baselines', {}) and 
                            obj in scenario_data[nc]['baselines'][original_name]):
                            algorithm_data[alg].append(scenario_data[nc]['baselines'][original_name][obj])
                        else:
                            algorithm_data[alg].append(0)
            
            # Create grouped bars
            x = range(len(node_counts))
            width = 0.15  # Width of each bar
            multiplier = 0
            
            for alg in all_algorithms:
                if any(v > 0 for v in algorithm_data[alg]):  # Only plot if we have data
                    offset = width * multiplier
                    color = algorithm_colors.get(alg, f'C{multiplier}')
                    bars = ax.bar([xi + offset for xi in x], algorithm_data[alg], width, 
                                 label=alg, color=color, alpha=0.8)
                    
                    multiplier += 1
            
            # Customize the plot
            ax.set_xlabel('Number of Nodes', fontsize=12)
            ax.set_ylabel(label, fontsize=12)
            ax.set_title(f'{label} Comparison', fontweight='bold')
            ax.set_xticks([xi + width * (len(all_algorithms) - 1) / 2 for xi in x])
            ax.set_xticklabels(node_counts)
            ax.grid(True, alpha=0.3, axis='y')
            
            # Format y-axis based on objective
            max_val = max([max(vals) for vals in algorithm_data.values() if any(v > 0 for v in vals)]) * 1.1
            ax.set_ylim(0, max_val)
            
            # Add "Lower is Better" at the top left
            ax.text(0.02, 0.98, 'Lower is Better', transform=ax.transAxes, 
                   fontsize=10, style='italic', va='top', ha='left',
                   bbox=dict(boxstyle='round,pad=0.3', facecolor='yellow', alpha=0.3))
            
            # Position legend below "Lower is Better" on the left for both charts
            ax.legend(fontsize=9, loc='upper left', bbox_to_anchor=(0.02, 0.85))
        
        plt.tight_layout()
        plt.savefig(output_path / 'academic_comparison.png', dpi=300, bbox_inches='tight')
        plt.close()
        print("  ✓ Created academic-style grouped bar comparison")
        print("  📊 Grouped bars make algorithm performance differences clearly visible")
        
    def plot_convergence_analysis(self, output_path: Path):
        """Plot convergence of objectives over optimization rounds."""
        fig, axes = plt.subplots(2, 2, figsize=(15, 12))
        fig.suptitle('Multi-Objective Optimization Process: Intermediate States (Pure Algorithm Performance)', fontsize=16, fontweight='bold')
        
        for i, result in enumerate(self.results):
            test_name = result['testCase']['name']
            rounds = result['rounds']
            
            if not rounds:
                continue
                
            # Extract optimization process data (initial → intermediate → final for each round)
            round_numbers = []
            initial_costs = []
            intermediate_costs = []
            final_costs = []
            initial_balances = []
            intermediate_balances = []
            final_balances = []
            
            for r in rounds:
                round_numbers.append(r['round'])
                
                # Raw cost values (actual $/hour)
                initial_costs.append(r['initialState']['totalCost'])
                intermediate_costs.append(r['intermediateState']['totalCost'])
                final_costs.append(r['finalState']['totalCost'])
                
                # Raw balance values (actual std dev %)
                initial_balances.append(r['initialState']['balancePercent'])
                intermediate_balances.append(r['intermediateState']['balancePercent'])
                final_balances.append(r['finalState']['balancePercent'])
            
            # Define colors for different scenarios
            colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b', '#e377c2', '#7f7f7f']
            color = colors[i % len(colors)]
            
            # Plot only intermediate optimization process (most relevant)
            axes[0, 0].plot(round_numbers, intermediate_costs, '-', linewidth=2, color=color,
                           label=f'{test_name}', alpha=0.8, marker='o', markersize=4)
            
            # Plot balance optimization process  
            axes[0, 1].plot(round_numbers, intermediate_balances, '-', linewidth=2, color=color,
                           label=f'{test_name}', alpha=0.8, marker='s', markersize=4)
            
            # Also plot the normalized disruption for comparison (shows NSGA-II optimization)
            disruptions = [r['bestSolution']['disruption'] for r in rounds]
            weighted_scores = [r['bestSolution']['weightedScore'] for r in rounds]
            
            axes[1, 0].plot(round_numbers, disruptions, marker='^', linewidth=2, color=color,
                           label=f'{test_name}', alpha=0.8, markersize=4)
            axes[1, 1].plot(round_numbers, weighted_scores, marker='d', linewidth=2, color=color,
                           label=f'{test_name}', alpha=0.8, markersize=4)
        
        # Configure subplots
        axes[0, 0].set_title('Cost Optimization (Intermediate States)', fontweight='bold')
        axes[0, 0].set_xlabel('Optimization Round')
        axes[0, 0].set_ylabel('Total Cluster Cost ($/hour)')
        axes[0, 0].grid(True, alpha=0.3)
        axes[0, 0].legend()
        
        axes[0, 1].set_title('Balance Optimization (Intermediate States)', fontweight='bold')
        axes[0, 1].set_xlabel('Optimization Round')
        axes[0, 1].set_ylabel('Load Balance Standard Deviation (%)')
        axes[0, 1].grid(True, alpha=0.3)
        axes[0, 1].legend()
        
        axes[1, 0].set_title('Disruption Convergence (NSGA-II)', fontweight='bold')
        axes[1, 0].set_xlabel('Optimization Round')
        axes[1, 0].set_ylabel('Normalized Disruption (0-1, lower is better)')
        axes[1, 0].grid(True, alpha=0.3)
        axes[1, 0].legend()
        
        axes[1, 1].set_title('Overall Weighted Score (NSGA-II)', fontweight='bold')
        axes[1, 1].set_xlabel('Optimization Round')
        axes[1, 1].set_ylabel('Weighted Objective Score (lower is better)')
        axes[1, 1].grid(True, alpha=0.3)
        axes[1, 1].legend()
        
        plt.tight_layout()
        plt.savefig(output_path / 'convergence_analysis.png', dpi=300, bbox_inches='tight')
        plt.close()
        print("  ✓ Created convergence analysis plot (showing optimization process: initial→intermediate→final)")
    
    def plot_pareto_front_evolution(self, output_path: Path):
        """Plot evolution of Pareto fronts across optimization rounds."""
        # Create subplots for different 2D projections of the 3D Pareto front
        fig, axes = plt.subplots(2, 2, figsize=(15, 12))
        fig.suptitle('Pareto Front Evolution in Objective Space', fontsize=18, fontweight='bold')
        
        colors = plt.cm.viridis(np.linspace(0, 1, 10))  # Up to 10 rounds
        
        for result in self.results:
            test_name = result['testCase']['name']
            rounds = result['rounds']
            
            if not rounds:
                continue
            
            # Plot evolution for each round
            for round_idx, round_data in enumerate(rounds[:5]):  # Show first 5 rounds
                pareto_front = round_data['paretoFront']
                
                if not pareto_front:
                    continue
                
                costs = [sol['rawCost'] for sol in pareto_front]
                balances = [sol['rawBalance'] for sol in pareto_front]
                disruptions = [sol['disruption'] for sol in pareto_front]
                movements = [sol['movements'] for sol in pareto_front]
                
                # Cost vs Balance
                axes[0, 0].scatter(costs, balances, c=[colors[round_idx]], 
                                  alpha=0.7, s=50, label=f'Round {round_data["round"]}')
                
                # Cost vs Disruption
                axes[0, 1].scatter(costs, disruptions, c=[colors[round_idx]], 
                                  alpha=0.7, s=50, label=f'Round {round_data["round"]}')
                
                # Balance vs Disruption
                axes[1, 0].scatter(balances, disruptions, c=[colors[round_idx]], 
                                  alpha=0.7, s=50, label=f'Round {round_data["round"]}')
                
                # Cost vs Movements
                axes[1, 1].scatter(costs, movements, c=[colors[round_idx]], 
                                  alpha=0.7, s=50, label=f'Round {round_data["round"]}')
        
        # Configure subplots
        axes[0, 0].set_xlabel('Cost ($/hour)')
        axes[0, 0].set_ylabel('Balance Standard Deviation (%)')
        axes[0, 0].set_title('Cost vs Balance Trade-off', fontweight='bold')
        axes[0, 0].grid(True, alpha=0.3)
        axes[0, 0].legend()
        
        axes[0, 1].set_xlabel('Cost ($/hour)')
        axes[0, 1].set_ylabel('Disruption (normalized)')
        axes[0, 1].set_title('Cost vs Disruption Trade-off', fontweight='bold')
        axes[0, 1].grid(True, alpha=0.3)
        axes[0, 1].legend()
        
        axes[1, 0].set_xlabel('Balance Standard Deviation (%)')
        axes[1, 0].set_ylabel('Disruption (normalized)')
        axes[1, 0].set_title('Balance vs Disruption Trade-off', fontweight='bold')
        axes[1, 0].grid(True, alpha=0.3)
        axes[1, 0].legend()
        
        axes[1, 1].set_xlabel('Cost ($/hour)')
        axes[1, 1].set_ylabel('Pod Movements')
        axes[1, 1].set_title('Cost vs Movement Count', fontweight='bold')
        axes[1, 1].grid(True, alpha=0.3)
        axes[1, 1].legend()
        
        plt.tight_layout()
        plt.savefig(output_path / 'pareto_front_evolution.png')
        plt.close()
        print("  ✓ Created Pareto front evolution plot")
    
    def plot_algorithm_comparison(self, output_path: Path):
        """Compare NSGA-II against baseline algorithms."""
        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('Algorithm Comparison: NSGA-II vs Baselines', fontsize=18, fontweight='bold')
        
        # Also create the academic-style grouped bar comparison
        self.plot_academic_bar_comparison(output_path)
        
        # Collect data for comparison
        algorithm_data = []
        
        for result in self.results:
            test_name = result['testCase']['name']
            baseline_results = result.get('baselineResults', [])
            comparison_metrics = result.get('comparisonMetrics', {})
            
            # Add NSGA-II result
            if 'finalResults' in result and result['finalResults']:
                nsga_best = comparison_metrics.get('nsgaiiBest', {})
                algorithm_data.append({
                    'test_case': test_name,
                    'algorithm': 'NSGA-II',
                    'cost': nsga_best.get('rawCost', 0),
                    'balance': nsga_best.get('rawBalance', 0),
                    'disruption': nsga_best.get('disruption', 0),
                    'weighted_score': nsga_best.get('weightedScore', 0),
                    'movements': nsga_best.get('movements', 0),
                    'feasible': True,
                    'execution_time': comparison_metrics.get('performanceComparison', {}).get('nsgaiiExecutionTime', 0)
                })
            
            # Add baseline results
            for baseline in baseline_results:
                algorithm_data.append({
                    'test_case': test_name,
                    'algorithm': baseline['algorithm'],
                    'cost': baseline['rawCost'],
                    'balance': baseline['rawBalance'],
                    'disruption': baseline['disruption'],
                    'weighted_score': baseline['weightedScore'],
                    'movements': baseline['movements'],
                    'feasible': baseline['feasible'],
                    'execution_time': baseline['executionTimeMs']
                })
        
        if not algorithm_data:
            print("  ⚠ No algorithm comparison data found")
            return
            
        df = pd.DataFrame(algorithm_data)
        
        # 1. Cost comparison (box plot)
        sns.boxplot(data=df, x='algorithm', y='cost', ax=axes[0, 0])
        axes[0, 0].set_title('Cost Comparison ($/hour)', fontweight='bold')
        axes[0, 0].set_xlabel('Algorithm')
        axes[0, 0].set_ylabel('Total Cluster Cost ($/hour)')
        axes[0, 0].tick_params(axis='x', rotation=45)
        
        # 2. Balance comparison (box plot)
        sns.boxplot(data=df, x='algorithm', y='balance', ax=axes[0, 1])
        axes[0, 1].set_title('Load Balance Comparison', fontweight='bold')
        axes[0, 1].set_xlabel('Algorithm')
        axes[0, 1].set_ylabel('Balance Standard Deviation (%)')
        axes[0, 1].tick_params(axis='x', rotation=45)
        
        # 3. Weighted score comparison (box plot)
        sns.boxplot(data=df, x='algorithm', y='weighted_score', ax=axes[1, 0])
        axes[1, 0].set_title('Weighted Score Comparison (Lower = Better)', fontweight='bold')
        axes[1, 0].set_xlabel('Algorithm')
        axes[1, 0].set_ylabel('Weighted Objective Score')
        axes[1, 0].tick_params(axis='x', rotation=45)
        
        # 4. Execution time comparison (log scale)
        sns.boxplot(data=df, x='algorithm', y='execution_time', ax=axes[1, 1])
        axes[1, 1].set_title('Execution Time Comparison', fontweight='bold')
        axes[1, 1].set_xlabel('Algorithm')
        axes[1, 1].set_ylabel('Execution Time (ms)')
        axes[1, 1].set_yscale('log')
        axes[1, 1].tick_params(axis='x', rotation=45)
        
        plt.tight_layout()
        plt.savefig(output_path / 'algorithm_comparison.png')
        plt.close()
        print("  ✓ Created algorithm comparison plot")
        
        # Create summary table
        self._create_algorithm_summary_table(df, output_path)
    
    def plot_academic_style_comparison_OLD(self, output_path: Path):
        """Create academic-style line plots comparing algorithms across different node counts."""
        # Group results by node count
        scenario_data = {}
        
        for result in self.results:
            test_case = result['testCase']
            node_count = len(test_case['nodes'])
            
            # Use node count as the x-axis variable
            if node_count not in scenario_data:
                scenario_data[node_count] = {
                    'nsga_ii': {},
                    'baselines': {}
                }
            
            # Extract NSGA-II final results from the last round
            if 'rounds' in result and result['rounds']:
                final_round = result['rounds'][-1]
                best_solution = final_round['bestSolution']
                
                scenario_data[node_count]['nsga_ii'] = {
                    'cost': best_solution['rawCost'],  # Use raw values for fair comparison
                    'balance': best_solution['rawBalance'],  # Use raw values for fair comparison 
                    'disruption': best_solution['disruption'],
                    'rounds_completed': len(result['rounds'])
                }
            
            # Extract baseline results
            if 'baselineResults' in result:
                for baseline in result['baselineResults']:
                    alg_name = baseline['algorithm']
                    if alg_name not in scenario_data[node_count]['baselines']:
                        scenario_data[node_count]['baselines'][alg_name] = {}
                    
                    scenario_data[node_count]['baselines'][alg_name] = {
                        'cost': baseline['rawCost'],
                        'balance': baseline['rawBalance'], 
                        'disruption': baseline['disruption'],
                        'execution_time': baseline.get('executionTimeMs', 0) / 1000.0
                    }
        
        if len(scenario_data) < 2:
            print("  ⚠ Not enough scenarios for academic-style comparison (need at least 2 different node counts)")
            return
            
        # Create the academic-style comparison plot
        fig, axes = plt.subplots(1, 3, figsize=(18, 6))
        fig.suptitle('Multi-Objective Optimization: Performance vs Cluster Size', fontsize=16, fontweight='bold')
        
        # Sort node counts for consistent x-axis
        node_counts = sorted(scenario_data.keys())
        print(f"📊 Creating academic comparison across {len(node_counts)} cluster sizes: {node_counts}")
        
        # Define algorithm colors and markers (academic style)
        algorithm_styles = {
            'NSGA-II': {'color': '#1f77b4', 'marker': 'o', 'linewidth': 3, 'markersize': 12, 'linestyle': '-', 'alpha': 1.0, 'offset': 0},
            'Best Fit Decreasing': {'color': '#ff7f0e', 'marker': 's', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '--', 'alpha': 0.9, 'offset': -0.5},
            'First Fit Decreasing': {'color': '#2ca02c', 'marker': '^', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '-.', 'alpha': 0.9, 'offset': 0.5},
            'Random Placement': {'color': '#d62728', 'marker': 'v', 'linewidth': 2.5, 'markersize': 10, 'linestyle': ':', 'alpha': 0.9, 'offset': -1.0},
            'Greedy Cost Scheduler': {'color': '#9467bd', 'marker': 'D', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '-', 'alpha': 0.9, 'offset': 1.0}
        }
        
        # Focus on the three main objectives
        objectives = ['cost', 'balance', 'disruption']
        objective_labels = ['Cost (USD/hour)', 'Load Balance (%)', 'Pod Movements']
        
        for i, (obj, label) in enumerate(zip(objectives, objective_labels)):
            ax = axes[i]
            
            # Plot NSGA-II (our method) with emphasis
            nsga_values = []
            for nc in node_counts:
                if 'nsga_ii' in scenario_data[nc] and obj in scenario_data[nc]['nsga_ii']:
                    nsga_values.append(scenario_data[nc]['nsga_ii'][obj])
                else:
                    nsga_values.append(0)
                    
            if any(v > 0 for v in nsga_values):
                style = algorithm_styles['NSGA-II']
                # Add slight offset to x-values to prevent overlap
                x_offset = [x + style.get('offset', 0) for x in node_counts]
                ax.plot(x_offset, nsga_values, 
                       color=style['color'], marker=style['marker'], 
                       linewidth=style['linewidth'], markersize=style['markersize'],
                       linestyle=style['linestyle'], label='NSGA-II (Proposed)', 
                       alpha=style.get('alpha', 0.9), zorder=5)
            
            # Plot baselines
            baseline_names = set()
            for nc in node_counts:
                if 'baselines' in scenario_data[nc]:
                    baseline_names.update(scenario_data[nc]['baselines'].keys())
            
            for alg_name in sorted(baseline_names):
                if alg_name in algorithm_styles:
                    baseline_values = []
                    for nc in node_counts:
                        if (alg_name in scenario_data[nc].get('baselines', {}) and 
                            obj in scenario_data[nc]['baselines'][alg_name]):
                            baseline_values.append(scenario_data[nc]['baselines'][alg_name][obj])
                        else:
                            baseline_values.append(0)
                    
                    if any(v > 0 for v in baseline_values):  # Only plot if we have data
                        style = algorithm_styles[alg_name]
                        # Add slight offset to x-values to prevent overlap
                        x_offset = [x + style.get('offset', 0) for x in node_counts]
                        ax.plot(x_offset, baseline_values,
                               color=style['color'], marker=style['marker'],
                               linewidth=style['linewidth'], markersize=style['markersize'],
                               linestyle=style.get('linestyle', '-'),
                               label=alg_name, alpha=style.get('alpha', 0.8), zorder=3)
            
            ax.set_xlabel('Number of Nodes', fontsize=12)
            ax.set_ylabel(label, fontsize=12)
            ax.grid(True, alpha=0.3)
            ax.legend(fontsize=10)
            
            # Format y-axis based on objective
            if obj == 'balance':
                ax.set_ylim(0, 100)
            elif obj in ['cost', 'disruption']:
                ax.set_ylim(bottom=0)
        
        plt.tight_layout()
        plt.savefig(output_path / 'academic_comparison.png', dpi=300, bbox_inches='tight')
        plt.close()
        print("  ✓ Created academic-style comparison plot")
    
    def plot_academic_style_comparison(self, output_path: Path):
        """Create academic-style line plots comparing algorithms across different node counts."""
        # Group results by node count
        scenario_data = {}
        
        for result in self.results:
            test_case = result['testCase']
            node_count = len(test_case['nodes'])
            
            # Use node count as the x-axis variable
            if node_count not in scenario_data:
                scenario_data[node_count] = {
                    'nsga_ii': {},
                    'baselines': {}
                }
            
            # Extract NSGA-II final results from the last round
            if 'rounds' in result and result['rounds']:
                final_round = result['rounds'][-1]
                best_solution = final_round['bestSolution']
                
                scenario_data[node_count]['nsga_ii'] = {
                    'cost': best_solution['rawCost'],  # Use raw values for fair comparison
                    'balance': best_solution['rawBalance'],  # Use raw values for fair comparison 
                    'disruption': best_solution['disruption'],
                    'rounds_completed': len(result['rounds'])
                }
            
            # Extract baseline results  
            if 'baselineResults' in result:
                for baseline in result['baselineResults']:
                    alg_name = baseline['algorithm']
                    if alg_name not in scenario_data[node_count]['baselines']:
                        scenario_data[node_count]['baselines'][alg_name] = {}
                    
                    scenario_data[node_count]['baselines'][alg_name] = {
                        'cost': baseline['rawCost'],
                        'balance': baseline['rawBalance'], 
                        'disruption': baseline['disruption'],
                        'execution_time': baseline.get('executionTimeMs', 0) / 1000.0
                    }
        
        if len(scenario_data) < 2:
            print("  ⚠ Not enough scenarios for academic-style comparison (need at least 2 different node counts)")
            return
            
        # Create the academic-style comparison plot
        fig, axes = plt.subplots(1, 3, figsize=(18, 6))
        fig.suptitle('Multi-Objective Optimization: Performance vs Cluster Size', fontsize=16, fontweight='bold')
        
        # Sort node counts for consistent x-axis
        node_counts = sorted(scenario_data.keys())
        print(f"📊 Creating academic comparison across {len(node_counts)} cluster sizes: {node_counts}")
        
        # Define algorithm colors and markers (academic style)
        algorithm_styles = {
            'NSGA-II': {'color': '#1f77b4', 'marker': 'o', 'linewidth': 3, 'markersize': 12, 'linestyle': '-', 'alpha': 1.0, 'offset': 0},
            'Best Fit Decreasing': {'color': '#ff7f0e', 'marker': 's', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '--', 'alpha': 0.9, 'offset': -0.5},
            'First Fit Decreasing': {'color': '#2ca02c', 'marker': '^', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '-.', 'alpha': 0.9, 'offset': 0.5},
            'Random Placement': {'color': '#d62728', 'marker': 'v', 'linewidth': 2.5, 'markersize': 10, 'linestyle': ':', 'alpha': 0.9, 'offset': -1.0},
            'Greedy Cost Scheduler': {'color': '#9467bd', 'marker': 'D', 'linewidth': 2.5, 'markersize': 10, 'linestyle': '-', 'alpha': 0.9, 'offset': 1.0}
        }
        
        # Focus on the three main objectives
        objectives = ['cost', 'balance', 'disruption']
        objective_labels = ['Cost (USD/hour)', 'Load Balance (%)', 'Pod Movements']
        
        for i, (obj, label) in enumerate(zip(objectives, objective_labels)):
            ax = axes[i]
            
            # Plot NSGA-II (our method) with emphasis
            nsga_values = []
            for nc in node_counts:
                if 'nsga_ii' in scenario_data[nc] and obj in scenario_data[nc]['nsga_ii']:
                    nsga_values.append(scenario_data[nc]['nsga_ii'][obj])
                else:
                    nsga_values.append(0)
                    
            if any(v > 0 for v in nsga_values):
                style = algorithm_styles['NSGA-II']
                # Add slight offset to x-values to prevent overlap
                x_offset = [x + style.get('offset', 0) for x in node_counts]
                ax.plot(x_offset, nsga_values, 
                       color=style['color'], marker=style['marker'], 
                       linewidth=style['linewidth'], markersize=style['markersize'],
                       linestyle=style['linestyle'], label='NSGA-II (Proposed)', 
                       alpha=style.get('alpha', 0.9), zorder=5)
            
            # Plot baselines
            baseline_names = set()
            for nc in node_counts:
                if 'baselines' in scenario_data[nc]:
                    baseline_names.update(scenario_data[nc]['baselines'].keys())
            
            for alg_name in sorted(baseline_names):
                if alg_name in algorithm_styles:
                    baseline_values = []
                    for nc in node_counts:
                        if (alg_name in scenario_data[nc].get('baselines', {}) and 
                            obj in scenario_data[nc]['baselines'][alg_name]):
                            baseline_values.append(scenario_data[nc]['baselines'][alg_name][obj])
                        else:
                            baseline_values.append(0)
                    
                    if any(v > 0 for v in baseline_values):  # Only plot if we have data
                        style = algorithm_styles[alg_name]
                        # Add slight offset to x-values to prevent overlap
                        x_offset = [x + style.get('offset', 0) for x in node_counts]
                        ax.plot(x_offset, baseline_values,
                               color=style['color'], marker=style['marker'],
                               linewidth=style['linewidth'], markersize=style['markersize'],
                               linestyle=style.get('linestyle', '-'),
                               label=alg_name, alpha=style.get('alpha', 0.8), zorder=3)
            
            ax.set_xlabel('Number of Nodes', fontsize=12)
            ax.set_ylabel(label, fontsize=12)
            ax.grid(True, alpha=0.3)
            ax.legend(fontsize=10)
            
            # Format y-axis based on objective
            if obj == 'balance':
                ax.set_ylim(0, 100)
            elif obj in ['cost', 'disruption']:
                ax.set_ylim(bottom=0)
        
        plt.tight_layout()
        plt.savefig(output_path / 'academic_comparison.png', dpi=300, bbox_inches='tight')
        plt.close()
        
        print(f"✅ Academic-style comparison plot saved to {output_path / 'academic_comparison.png'}")
        
        # Create convergence plot showing pod movements over rounds
        self.plot_convergence_behavior(output_path)
        
        # Also create a summary table showing the data
        print(f"\n📊 Algorithm Performance Summary:")
        print(f"{'Nodes':<8} {'Algorithm':<20} {'Cost':<8} {'Balance':<8} {'Movements':<10}")
        print("-" * 60)
        for nc in node_counts:
            # NSGA-II
            if 'nsga_ii' in scenario_data[nc]:
                nsga = scenario_data[nc]['nsga_ii']
                print(f"{nc:<8} {'NSGA-II':<20} {nsga.get('cost', 0):<8.2f} {nsga.get('balance', 0):<8.1f} {nsga.get('disruption', 0):<10.0f}")
            
            # Best baseline for comparison
            if scenario_data[nc]['baselines']:
                best_baseline = min(scenario_data[nc]['baselines'].items(), 
                                  key=lambda x: x[1].get('cost', float('inf')))
                baseline_name, baseline_data = best_baseline
                print(f"{nc:<8} {baseline_name:<20} {baseline_data.get('cost', 0):<8.2f} {baseline_data.get('balance', 0):<8.1f} {baseline_data.get('disruption', 0):<10.0f}")
            print()
    
    def plot_convergence_behavior(self, output_path: Path):
        """Plot convergence behavior showing pod movements over rounds for different cluster sizes."""
        # Group results by node count and extract round-by-round data
        convergence_data = {}
        
        for result in self.results:
            test_case = result['testCase']
            node_count = len(test_case['nodes'])
            
            if 'rounds' in result and result['rounds']:
                movements_per_round = []
                feasible_movements_per_round = []
                
                for round_data in result['rounds']:
                    # Extract movement data from each round
                    if 'feasibleMoves' in round_data:
                        # The JSON structure has nested 'feasibleMoves' - the outer is the object, inner is the count
                        feasible_data = round_data['feasibleMoves']
                        feasible_count = feasible_data.get('feasibleMoves', 0)  # Actual moved pods
                        total_target = feasible_data.get('totalTargetMoves', 0)  # Total desired moves
                        feasible_movements_per_round.append(feasible_count)
                        movements_per_round.append(total_target)
                    else:
                        # Fallback: try to get from best solution
                        if 'bestSolution' in round_data:
                            movements = round_data['bestSolution'].get('disruption', 0)
                            movements_per_round.append(movements)
                            feasible_movements_per_round.append(movements)  # Assume all are feasible as fallback
                
                convergence_data[node_count] = {
                    'movements': movements_per_round,
                    'feasible_movements': feasible_movements_per_round,
                    'rounds': len(movements_per_round),
                    'convergence_reason': result.get('finalResults', {}).get('convergenceReason', 'Unknown')
                }
        
        if not convergence_data:
            print("  ⚠ No convergence data available for plotting")
            return
        
        # Create convergence plots
        fig, axes = plt.subplots(1, 2, figsize=(16, 6))
        fig.suptitle('NSGA-II Convergence Behavior: Pod Movements Over Rounds', fontsize=16, fontweight='bold')
        
        # Sort node counts for consistent colors
        node_counts = sorted(convergence_data.keys())
        colors = plt.cm.viridis(np.linspace(0, 1, len(node_counts)))
        
        # Plot 1: Feasible movements per round
        ax1 = axes[0]
        for i, nc in enumerate(node_counts):
            data = convergence_data[nc]
            rounds = list(range(1, len(data['feasible_movements']) + 1))
            
            ax1.plot(rounds, data['feasible_movements'], 
                    color=colors[i], marker='o', linewidth=2, markersize=6,
                    label=f'{nc} nodes ({data["rounds"]} rounds)', alpha=0.8)
            
            # Mark the convergence point if it converged early
            if 'PDB-blocked' in data['convergence_reason']:
                # Find where it hits zero and stays zero
                zero_streaks = []
                current_streak = 0
                for j, moves in enumerate(data['feasible_movements']):
                    if moves == 0:
                        current_streak += 1
                    else:
                        if current_streak >= 3:  # Mark significant zero streaks
                            zero_streaks.append((j - current_streak + 1, j))
                        current_streak = 0
                
                # Mark the final convergence point
                if zero_streaks:
                    conv_start = zero_streaks[-1][0] + 1  # +1 for 1-based indexing
                    ax1.axvline(x=conv_start, color=colors[i], linestyle='--', alpha=0.5)
                    ax1.text(conv_start, max(data['feasible_movements']) * 0.8, 
                            'PDB\nBlocked', rotation=90, ha='center', va='top',
                            fontsize=9, color=colors[i], alpha=0.7)
        
        ax1.set_xlabel('Round Number', fontsize=12)
        ax1.set_ylabel('Feasible Pod Movements', fontsize=12)
        ax1.set_title('Feasible Movements per Round', fontweight='bold')
        ax1.grid(True, alpha=0.3)
        ax1.legend()
        ax1.set_ylim(bottom=0)
        
        # Plot 2: Cumulative pod movements (showing total optimization effort)
        ax2 = axes[1] 
        for i, nc in enumerate(node_counts):
            data = convergence_data[nc]
            rounds = list(range(1, len(data['feasible_movements']) + 1))
            cumulative_movements = np.cumsum(data['feasible_movements'])
            
            ax2.plot(rounds, cumulative_movements,
                    color=colors[i], marker='s', linewidth=2, markersize=5,
                    label=f'{nc} nodes (total: {cumulative_movements[-1] if len(cumulative_movements) > 0 else 0})', alpha=0.8)
        
        ax2.set_xlabel('Round Number', fontsize=12)
        ax2.set_ylabel('Cumulative Pod Movements', fontsize=12)
        ax2.set_title('Total Optimization Effort', fontweight='bold')
        ax2.grid(True, alpha=0.3)
        ax2.legend()
        ax2.set_ylim(bottom=0)
        
        plt.tight_layout()
        plt.savefig(output_path / 'convergence_behavior.png', dpi=300, bbox_inches='tight')
        plt.close()
        
        print(f"✅ Convergence behavior plot saved to {output_path / 'convergence_behavior.png'}")
        
        # Print convergence summary
        print(f"\n📊 Convergence Analysis:")
        print(f"{'Nodes':<8} {'Rounds':<8} {'Total Moves':<12} {'Final 3 Rounds':<15} {'Convergence':<20}")
        print("-" * 75)
        for nc in node_counts:
            data = convergence_data[nc]
            total_moves = sum(data['feasible_movements']) if data['feasible_movements'] else 0
            final_3 = data['feasible_movements'][-3:] if len(data['feasible_movements']) >= 3 else data['feasible_movements']
            final_3_str = f"{final_3}" if len(final_3) <= 3 else f"{final_3[-3:]}"
            convergence_type = "PDB-blocked" if "PDB-blocked" in data['convergence_reason'] else "Completed"
            
            print(f"{nc:<8} {data['rounds']:<8} {total_moves:<12} {final_3_str:<15} {convergence_type:<20}")
    
    def _create_algorithm_summary_table(self, df: pd.DataFrame, output_path: Path):
        """Create a summary table of algorithm performance."""
        summary = df.groupby('algorithm').agg({
            'cost': ['mean', 'std', 'min'],
            'balance': ['mean', 'std', 'min'],
            'weighted_score': ['mean', 'std', 'min'],
            'execution_time': ['mean', 'std'],
            'feasible': 'sum'
        }).round(3)
        
        # Flatten column names
        summary.columns = ['_'.join(col).strip() for col in summary.columns]
        
        # Save to CSV
        summary.to_csv(output_path / 'algorithm_summary.csv')
        print("  ✓ Created algorithm summary table (CSV)")
    
    def plot_tradeoff_analysis(self, output_path: Path):
        """Create parallel coordinates plot for trade-off analysis."""
        fig, ax = plt.subplots(1, 1, figsize=(14, 8))
        
        # Collect Pareto solutions from all results
        pareto_solutions = []
        
        for result in self.results:
            test_name = result['testCase']['name']
            rounds = result['rounds']
            
            if not rounds:
                continue
            
            # Use final round Pareto front
            final_round = rounds[-1]
            pareto_front = final_round['paretoFront']
            
            for sol in pareto_front:
                pareto_solutions.append({
                    'test_case': test_name,
                    'cost': sol['rawCost'],
                    'balance': sol['rawBalance'],
                    'disruption': sol['disruption'],
                    'movements': sol['movements'],
                    'weighted_score': sol['weightedScore']
                })
        
        if not pareto_solutions:
            print("  ⚠ No Pareto solutions found for trade-off analysis")
            return
            
        df = pd.DataFrame(pareto_solutions)
        
        # Normalize data for parallel coordinates
        normalized_df = df.copy()
        for col in ['cost', 'balance', 'disruption', 'movements']:
            if col in normalized_df.columns and normalized_df[col].std() > 0:
                normalized_df[col] = (normalized_df[col] - normalized_df[col].min()) / (normalized_df[col].max() - normalized_df[col].min())
        
        # Create parallel coordinates plot
        from pandas.plotting import parallel_coordinates
        parallel_coordinates(normalized_df, 'test_case', colormap='viridis', alpha=0.7, ax=ax)
        
        ax.set_title('Multi-Objective Trade-off Analysis\n(Normalized Pareto Solutions)', 
                    fontweight='bold', fontsize=16)
        ax.set_ylabel('Normalized Objective Values')
        ax.grid(True, alpha=0.3)
        ax.legend(bbox_to_anchor=(1.05, 1), loc='upper left')
        
        plt.tight_layout()
        plt.savefig(output_path / 'tradeoff_analysis.png')
        plt.close()
        print("  ✓ Created trade-off analysis plot")
    
    def plot_performance_comparison(self, output_path: Path):
        """Plot performance comparison between algorithms."""
        fig, axes = plt.subplots(1, 2, figsize=(15, 6))
        fig.suptitle('Algorithm Performance Analysis', fontsize=18, fontweight='bold')
        
        improvement_data = []
        performance_data = []
        
        for result in self.results:
            test_name = result['testCase']['name']
            comparison = result.get('comparisonMetrics', {})
            
            if comparison:
                improvement_data.append({
                    'test_case': test_name,
                    'improvement_ratio': comparison.get('improvementRatio', 1),
                    'cost_improvement': comparison.get('costImprovement', 0),
                    'balance_improvement': comparison.get('balanceImprovement', 0)
                })
                
                perf_comp = comparison.get('performanceComparison', {})
                performance_data.append({
                    'test_case': test_name,
                    'nsga_time': perf_comp.get('nsgaiiExecutionTime', 0),
                    'baseline_time': perf_comp.get('fastestBaselineTime', 0),
                    'speedup_ratio': perf_comp.get('speedupRatio', 1),
                    'fastest_baseline': perf_comp.get('fastestBaseline', 'Unknown')
                })
        
        if improvement_data:
            df_improvement = pd.DataFrame(improvement_data)
            
            # Improvement ratio plot
            axes[0].bar(df_improvement['test_case'], df_improvement['improvement_ratio'], 
                       color='green', alpha=0.7)
            axes[0].axhline(y=1, color='red', linestyle='--', alpha=0.8, label='No improvement')
            axes[0].set_title('Solution Quality Improvement\n(NSGA-II vs Best Baseline)', fontweight='bold')
            axes[0].set_xlabel('Test Case')
            axes[0].set_ylabel('Improvement Ratio')
            axes[0].tick_params(axis='x', rotation=45)
            axes[0].legend()
            axes[0].grid(True, alpha=0.3)
        
        if performance_data:
            df_performance = pd.DataFrame(performance_data)
            
            # Performance comparison plot
            x_pos = np.arange(len(df_performance))
            width = 0.35
            
            axes[1].bar(x_pos - width/2, df_performance['nsga_time'], width, 
                       label='NSGA-II', color='blue', alpha=0.7)
            axes[1].bar(x_pos + width/2, df_performance['baseline_time'], width,
                       label='Fastest Baseline', color='orange', alpha=0.7)
            
            axes[1].set_title('Execution Time Comparison', fontweight='bold')
            axes[1].set_xlabel('Test Case')
            axes[1].set_ylabel('Execution Time (ms)')
            axes[1].set_yscale('log')
            axes[1].set_xticks(x_pos)
            axes[1].set_xticklabels(df_performance['test_case'], rotation=45)
            axes[1].legend()
            axes[1].grid(True, alpha=0.3)
        
        plt.tight_layout()
        plt.savefig(output_path / 'performance_comparison.png')
        plt.close()
        print("  ✓ Created performance comparison plot")
    
    def plot_statistical_analysis(self, output_path: Path):
        """Create statistical analysis plots."""
        fig, axes = plt.subplots(2, 2, figsize=(15, 12))
        fig.suptitle('Statistical Analysis of Optimization Results', fontsize=18, fontweight='bold')
        
        # Collect data for statistical analysis
        cost_data = []
        balance_data = []
        movement_data = []
        
        for result in self.results:
            test_name = result['testCase']['name']
            rounds = result['rounds']
            
            for round_data in rounds:
                cost_data.append({
                    'test_case': test_name,
                    'round': round_data['round'],
                    'cost': round_data['bestSolution']['rawCost'],
                    'cost_savings': round_data['improvements']['costSavings']
                })
                
                balance_data.append({
                    'test_case': test_name,
                    'round': round_data['round'],
                    'balance': round_data['bestSolution']['rawBalance'],
                    'balance_improvement': round_data['improvements']['balanceImprovement']
                })
                
                movement_data.append({
                    'test_case': test_name,
                    'round': round_data['round'],
                    'total_moves': round_data['feasibleMoves']['totalTargetMoves'],
                    'feasible_moves': round_data['feasibleMoves']['feasibleMoves'],
                    'feasibility_percent': round_data['feasibleMoves']['feasibilityPercent']
                })
        
        if cost_data:
            df_cost = pd.DataFrame(cost_data)
            
            # Cost improvement distribution
            axes[0, 0].hist(df_cost['cost_savings'], bins=20, alpha=0.7, color='green', edgecolor='black')
            axes[0, 0].set_title('Distribution of Cost Savings', fontweight='bold')
            axes[0, 0].set_xlabel('Cost Savings ($/hour)')
            axes[0, 0].set_ylabel('Frequency')
            axes[0, 0].grid(True, alpha=0.3)
        
        if balance_data:
            df_balance = pd.DataFrame(balance_data)
            
            # Balance improvement distribution
            axes[0, 1].hist(df_balance['balance_improvement'], bins=20, alpha=0.7, color='blue', edgecolor='black')
            axes[0, 1].set_title('Distribution of Balance Improvements', fontweight='bold')
            axes[0, 1].set_xlabel('Balance Improvement (%)')
            axes[0, 1].set_ylabel('Frequency')
            axes[0, 1].grid(True, alpha=0.3)
        
        if movement_data:
            df_movement = pd.DataFrame(movement_data)
            
            # PDB feasibility analysis
            axes[1, 0].scatter(df_movement['total_moves'], df_movement['feasibility_percent'], 
                              alpha=0.6, s=50)
            axes[1, 0].set_title('PDB Constraint Impact on Feasibility', fontweight='bold')
            axes[1, 0].set_xlabel('Total Target Movements')
            axes[1, 0].set_ylabel('Feasibility Percentage (%)')
            axes[1, 0].grid(True, alpha=0.3)
            
            # Feasible vs total movements
            axes[1, 1].scatter(df_movement['total_moves'], df_movement['feasible_moves'], 
                              alpha=0.6, s=50, color='orange')
            # Add diagonal line for perfect feasibility
            max_moves = max(df_movement['total_moves']) if not df_movement['total_moves'].empty else 1
            axes[1, 1].plot([0, max_moves], [0, max_moves], 'r--', alpha=0.8, label='Perfect Feasibility')
            axes[1, 1].set_title('Feasible vs Target Movements', fontweight='bold')
            axes[1, 1].set_xlabel('Total Target Movements')
            axes[1, 1].set_ylabel('Feasible Movements')
            axes[1, 1].legend()
            axes[1, 1].grid(True, alpha=0.3)
        
        plt.tight_layout()
        plt.savefig(output_path / 'statistical_analysis.png')
        plt.close()
        print("  ✓ Created statistical analysis plot")
    
    def plot_scalability_analysis(self, output_path: Path):
        """Analyze scalability across different cluster sizes."""
        scalability_data = []
        
        for result in self.results:
            test_case = result['testCase']
            num_nodes = len(test_case['nodes'])
            num_pods = len(test_case['pods'])
            
            # Calculate cluster utilization
            # Handle both lowercase and uppercase field names (Go JSON uses uppercase)
            total_cpu_capacity = sum(node.get('cpu', node.get('CPU', 0)) for node in test_case['nodes'])
            total_mem_capacity = sum(node.get('mem', node.get('Mem', 0)) for node in test_case['nodes'])
            total_cpu_request = sum(pod.get('cpu', pod.get('CPU', 0)) for pod in test_case['pods'])
            total_mem_request = sum(pod.get('mem', pod.get('Mem', 0)) for pod in test_case['pods'])
            
            cpu_utilization = (total_cpu_request / total_cpu_capacity) * 100 if total_cpu_capacity > 0 else 0
            mem_utilization = (total_mem_request / total_mem_capacity) * 100 if total_mem_capacity > 0 else 0
            
            # Get final results
            if result.get('finalResults'):
                final_results = result['finalResults']
                scalability_data.append({
                    'test_case': test_case['name'],
                    'num_nodes': num_nodes,
                    'num_pods': num_pods,
                    'cluster_size': num_nodes * num_pods,  # Simple size metric
                    'cpu_utilization': cpu_utilization,
                    'mem_utilization': mem_utilization,
                    'total_rounds': final_results['totalRounds'],
                    'final_pareto_size': final_results['finalParetoSize'],
                    'total_cost_savings': final_results['totalCostSavings']
                })
        
        if len(scalability_data) < 2:
            print("  ⚠ Not enough data points for scalability analysis")
            return
            
        df = pd.DataFrame(scalability_data)
        
        fig, axes = plt.subplots(2, 2, figsize=(15, 12))
        fig.suptitle('Scalability Analysis Across Cluster Sizes', fontsize=18, fontweight='bold')
        
        # Convergence vs cluster size
        axes[0, 0].scatter(df['cluster_size'], df['total_rounds'], s=100, alpha=0.7, color='blue')
        axes[0, 0].set_title('Convergence vs Cluster Size', fontweight='bold')
        axes[0, 0].set_xlabel('Cluster Size (nodes × pods)')
        axes[0, 0].set_ylabel('Rounds to Convergence')
        axes[0, 0].grid(True, alpha=0.3)
        
        # Pareto front size vs cluster size
        axes[0, 1].scatter(df['cluster_size'], df['final_pareto_size'], s=100, alpha=0.7, color='green')
        axes[0, 1].set_title('Pareto Front Size vs Cluster Size', fontweight='bold')
        axes[0, 1].set_xlabel('Cluster Size (nodes × pods)')
        axes[0, 1].set_ylabel('Final Pareto Front Size')
        axes[0, 1].grid(True, alpha=0.3)
        
        # Cost savings vs utilization
        axes[1, 0].scatter(df['cpu_utilization'], df['total_cost_savings'], s=100, alpha=0.7, color='red')
        axes[1, 0].set_title('Cost Savings vs CPU Utilization', fontweight='bold')
        axes[1, 0].set_xlabel('CPU Utilization (%)')
        axes[1, 0].set_ylabel('Total Cost Savings ($/hour)')
        axes[1, 0].grid(True, alpha=0.3)
        
        # Memory vs CPU utilization
        axes[1, 1].scatter(df['cpu_utilization'], df['mem_utilization'], s=100, alpha=0.7, color='purple')
        axes[1, 1].set_title('Memory vs CPU Utilization', fontweight='bold')
        axes[1, 1].set_xlabel('CPU Utilization (%)')
        axes[1, 1].set_ylabel('Memory Utilization (%)')
        axes[1, 1].grid(True, alpha=0.3)
        
        plt.tight_layout()
        plt.savefig(output_path / 'scalability_analysis.png')
        plt.close()
        print("  ✓ Created scalability analysis plot")


def main():
    """Main function to parse arguments and run analysis."""
    parser = argparse.ArgumentParser(
        description='Analyze multi-objective optimization results',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python analyze_optimization_results.py optimization_results_*.json
  python analyze_optimization_results.py results/test1.json results/test2.json
  python analyze_optimization_results.py --output-dir paper_figures optimization_results_*.json
        """
    )
    
    parser.add_argument('files', nargs='+', help='JSON result files to analyze')
    parser.add_argument('--output-dir', '-o', default='optimization_analysis', 
                       help='Output directory for visualizations (default: optimization_analysis)')
    
    args = parser.parse_args()
    
    # Expand glob patterns
    json_files = []
    for pattern in args.files:
        expanded = glob.glob(pattern)
        if expanded:
            json_files.extend(expanded)
        else:
            json_files.append(pattern)  # Keep as-is if no glob expansion
    
    if not json_files:
        print("❌ No JSON files found!")
        sys.exit(1)
    
    print(f"🚀 Multi-Objective Optimization Results Analyzer")
    print(f"📊 Found {len(json_files)} result files to analyze")
    
    # Create analyzer and run analysis
    analyzer = OptimizationAnalyzer(json_files)
    
    if not analyzer.results:
        print("❌ No valid result files could be loaded!")
        sys.exit(1)
    
    analyzer.create_all_visualizations(args.output_dir)
    
    print(f"\n✅ Analysis complete! Check {args.output_dir}/ for all visualizations.")
    print(f"📈 Generated visualizations suitable for academic publications.")


if __name__ == '__main__':
    main()
